package podstate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimgatz98/k8s-gpu-scheduler/utils"
	"github.com/go-redis/redis/v8"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/resources"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/redis/client"

	promMetrics "github.com/dimgatz98/k8s-gpu-scheduler/pkg/prom/fetch_prom_metrics"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	grpcClient "github.com/dimgatz98/k8s-gpu-scheduler/pkg/recommender/go_client/pkg"
)

type GPU struct {
	handle framework.Handle
}

var _ = framework.ScorePlugin(&GPU{})
var _ = framework.PostBindPlugin(&GPU{})

var mu sync.Mutex
var count int = 0
var clientset *kubernetes.Clientset = nil

const Name = "GPU"

var configs []string = []string{"all-1g.6gb", "all-2g.12gb", "all-4g.24gb"}
var partitions []int = []int{4, 2, 1}
var configCount = 0

type Pod struct {
	Name      string
	Namespace string
}

type BoundNode struct {
	Name string
	UUID string
}

func (g *GPU) Name() string {
	return Name
}

type RedisScore struct {
	NodeName string
	Uuid     string
	Score    int64
}

func GetSLOs(nodeName string, uuids []string, clientset *kubernetes.Clientset, redisDesc *client.Descriptor) (res map[string]map[Pod]float32, err error) {
	res = map[string]map[Pod]float32{}

	podsDesc, err := resources.New("", "", "", clientset)
	if err != nil {
		return nil, err
	}
	podsList, err := podsDesc.ListPods()
	if err != nil {
		return nil, err
	}
	podsSet := map[Pod]int{}
	for i, pod := range podsList.Items {
		podsSet[Pod{Name: pod.GetName(), Namespace: pod.GetNamespace()}] = i
	}
	for _, uuid := range uuids {
		boundNodeBytes, err := json.Marshal(BoundNode{Name: nodeName, UUID: uuid})
		if err != nil {
			return nil, err
		}
		collocatedPodsListBytes, err := redisDesc.Get(string(boundNodeBytes))
		var collocatedPodsList []Pod
		json.Unmarshal([]byte(collocatedPodsListBytes), &collocatedPodsList)
		if err != nil {
			return nil, err
		}

		runningPods := []Pod{}
		for _, pod := range collocatedPodsList {
			val, ok := podsSet[pod]
			if ok && (podsList.Items[val].Status.Phase == corev1.PodRunning) {
				runningPods = append(runningPods, pod)
			}
		}

		SLO := map[Pod]float32{}
		for _, pod := range runningPods {
			ind := podsSet[pod]
			value := utils.GetEnv(&podsList.Items[ind], "SLO")
			floatSLO, err := strconv.ParseFloat(value, 32)
			if err != nil {
				return nil, err
			}
			SLO[pod] = float32(floatSLO)
		}
		res[uuid] = SLO
	}

	return res, nil
}

func GetDcgmMetricsForUUIDS(nodeName string, clientset *kubernetes.Clientset, podList *corev1.PodList, uuids []string) (metrics map[string]map[string]float32, err error) {
	dcgmPod, err := utils.GetNodesDcgmPod(nodeName, podList, clientset)
	log.Println("dcgmPod: ", dcgmPod)
	utils.Check(err)

	// Get prometheus node's IP
	promUrls, err := utils.FindNodesIPFromPod("prometheus-0", "prometheus", "", clientset, nil)
	log.Println("promUrls: ", promUrls)
	utils.Check(err)
	if promUrls == nil {
		klog.Fatal("Prometheus not found")
	}
	var promUrl string
	for _, value := range promUrls[0] {
		promUrl = value
		break
	}

	// Fetch DCGM data and score node based on the data
	responses, err := promMetrics.DcgmPromInstantQuery(
		fmt.Sprintf("http://%s:30090/", promUrl),
		fmt.Sprintf("{pod=\"%s\"}", dcgmPod),
	)
	if responses == nil {
		return nil, err
	}

	isMig := utils.Exists(uuids, "MIG")
	gpuIds := []string{}
	if isMig != -1 {
		for _, response := range responses {
			gpuIds = append(gpuIds, response.GPU_I_ID)
		}
	}
	sort.Slice(gpuIds, func(i, j int) bool {
		return i > j
	})
	gpuIdToUUID := map[string]string{}
	for i, gpuId := range gpuIds {
		gpuIdToUUID[gpuId] = uuids[i]
	}

	utils.Check(err)
	metrics = make(map[string]map[string]float32)
	for _, response := range responses {
		value, err := strconv.ParseFloat(response.Value, 32)
		utils.Check(err)
		uuid := response.UUID
		gpuId := response.GPU_I_ID
		var key string
		if gpuId != "" {
			key = gpuIdToUUID[gpuId]
		} else {
			key = uuid
		}

		if _, found := metrics[key]; !found {
			metrics[key] = map[string]float32{}
		}
		metrics[key][response.MetricName] = float32(value)
		klog.Info(response.MetricName, " for node {", nodeName, "} = ", response.Value)
	}
	return metrics, nil
}

func GetDcgmMetricsForNode(nodeName string, clientset *kubernetes.Clientset, podList *corev1.PodList) (metrics map[string]map[string]float32, err error) {
	if podList == nil {
		desc, err := resources.New("", "spec.nodeName="+nodeName, "", clientset)
		if err != nil {
			return nil, err
		}
		podList, err = desc.ListPods()
		if err != nil {
			return nil, err
		}
	}

	dcgmPod, err := utils.GetNodesDcgmPod(nodeName, podList, clientset)
	log.Println("dcgmPod: ", dcgmPod)
	utils.Check(err)

	// Get prometheus node's IP
	promUrls, err := utils.FindNodesIPFromPod("prometheus-0", "", "", clientset, podList)
	log.Println("promUrls: ", promUrls)
	utils.Check(err)
	if promUrls == nil {
		klog.Fatal("Prometheus not found")
	}
	var promUrl string
	for _, value := range promUrls[0] {
		promUrl = value
		break
	}

	// Fetch DCGM data and score node based on the data
	responses, err := promMetrics.DcgmPromInstantQuery(
		fmt.Sprintf("http://%s:30090/", promUrl),
		fmt.Sprintf("{pod=\"%s\"}", dcgmPod),
	)
	if responses == nil {
		return nil, err
	}

	utils.Check(err)
	metrics = make(map[string]map[string]float32)
	for _, response := range responses {
		value, err := strconv.ParseFloat(response.Value, 32)
		utils.Check(err)
		uuid := response.UUID
		gpuId := response.GPU_I_ID
		var key string
		if gpuId != "" {
			key = gpuId
		} else {
			key = uuid
		}

		if _, found := metrics[key]; !found {
			metrics[key] = map[string]float32{}
		}
		metrics[key][response.MetricName] = float32(value)
		klog.Info(response.MetricName, " for node {", nodeName, "} = ", response.Value)
	}
	return metrics, nil
}

func GetConfigurationPredictions(podName string, podList *corev1.PodList) (configurations map[string]float32, err error) {
	// Find recommender node's IP
	recommenderIPs, err := utils.FindNodesIPFromPod("recommender", "", "", clientset, podList)
	utils.Check(err)
	if recommenderIPs == nil {
		return nil, err
	}
	var recommenderIP string
	for _, value := range recommenderIPs[0] {
		recommenderIP = value
		break
	}

	// Looking for recommendations
	response, err := grpcClient.ImputeConfigurations(recommenderIP+":32700", podName)
	if err != nil {
		return nil, err
	}

	configurations = map[string]float32{}
	for i, configuration := range response.Columns {
		configurations[configuration] = response.Result[i]
	}

	return configurations, nil
}

func GetInterferencePredictions(podName string, podList *corev1.PodList) (interference map[string]float32, err error) {
	// Find recommender node's IP
	recommenderIPs, err := utils.FindNodesIPFromPod("recommender", "", "", clientset, podList)
	utils.Check(err)
	if recommenderIPs == nil {
		return nil, err
	}
	var recommenderIP string
	for _, value := range recommenderIPs[0] {
		recommenderIP = value
		break
	}

	// Looking for recommendations
	response, err := grpcClient.ImputeInterference(recommenderIP+":32700", podName)
	if err != nil {
		return nil, err
	}

	interference = map[string]float32{}
	for i, pod := range response.Columns {
		interference[pod] = response.Result[i]
	}

	return interference, nil
}

func Logic(nodeName string, pod *v1.Pod, clientset *kubernetes.Clientset) (int64, error) {
	var err error
	var score int64 = framework.MinNodeScore
	var selectedUUID string = ""
	var currSLO float64

	tmpSLO := utils.GetEnv(pod, "SLO")
	if tmpSLO == "" {
		currSLO = 0
	} else {
		currSLO, err = strconv.ParseFloat(tmpSLO, 32)
		if err != nil {
			currSLO = 0
		}
	}

	redisUrls, err := utils.FindNodesIPFromPod("-0", "redis", "", clientset, nil)
	if err != nil {
		klog.Info("FindNodesIP() failed in GetSLOs: ", err.Error())
	}
	if redisUrls == nil {
		metrics, err := GetDcgmMetricsForNode(nodeName, clientset, nil)
		if err != nil {
			return 0, err
		}
		if len(metrics) == 0 {
			return 0, fmt.Errorf("empty dcgm metrics")
		}

		for key := range metrics {
			util := metrics[key]["DCGM_FI_PROF_GR_ENGINE_ACTIVE"]
			fbFree := metrics[key]["DCGM_FI_DEV_FB_FREE"]
			tmpScore := fbFree - util*fbFree
			if int64(tmpScore) > score {
				score = int64(tmpScore)
			}
		}
		klog.Warning("redisUrls empty in Logic()")
		return score, nil
	} else {
		var redisUrl string
		for _, value := range redisUrls[0] {
			redisUrl = value
		}
		// Add redis port and password in k8s secret
		redisDesc := client.New(redisUrl+":32767", "1234", 0)

		uuids, err := redisDesc.Get(nodeName)
		if err != nil && !errors.Is(err, redis.Nil) {
			klog.Warning("error in redisDesc.Get() in Logic(): ", err)
			return 0, err
		}
		var tmpUuids []string
		json.Unmarshal([]byte(uuids), &tmpUuids)

		SLOs, err := GetSLOs(nodeName, tmpUuids, clientset, redisDesc)
		if err != nil && !errors.Is(err, redis.Nil) {
			klog.Warningf("error in GetSLOs() in Logic(): %+v", SLOs, err)
			return 0, err
		}
		klog.Infof("SLOs: %+v", SLOs)

		metrics, err := GetDcgmMetricsForUUIDS(nodeName, clientset, nil, tmpUuids)
		if err != nil {
			return 0, err
		}

		nodeModel := ""
		if strings.Contains(nodeName, "a30") {
			nodeModel = "A30"
		} else if strings.Contains(nodeName, "gpu") {
			nodeModel = "V100"
		}

		klog.Info("nodeName: ", nodeName, "\nNodeModel: ", nodeModel)

		if nodeModel != "" {
			for _, uuid := range tmpUuids {
				var sum float64 = 1
				_, ok := SLOs[uuid]
				// If the key exists
				if ok {
					for pod, SLO := range SLOs[uuid] {
						if SLO == 0 {
							continue
						}
						confIndex := ""
						confPredictions, err := GetConfigurationPredictions(pod.Name, nil)
						if err != nil {
							return 0, err
						}

						confIndex = fmt.Sprintf("%dP_%s", len(tmpUuids), nodeModel)
						confPrediction, ok := confPredictions[confIndex]
						if !ok {
							continue
						}

						var interference float32 = 0
						tmpInterference, err := GetInterferencePredictions(pod.Name, nil)
						if err != nil {
							return 0, err
						}
						for collocatedPod := range SLOs[uuid] {
							if pod == collocatedPod {
								continue
							}

							for tmpPod, val := range tmpInterference {
								if strings.Contains(collocatedPod.Name, tmpPod) {
									interference += val
									break
								}
							}
						}
						for tmpPod, val := range tmpInterference {
							if strings.Contains(pod.Name, tmpPod) {
								interference += val
								break
							}
						}

						sum += 1 / (math.Abs(float64(SLO - (confPrediction - interference))))
					}
				}

				confIndex := ""
				confPredictions, err := GetConfigurationPredictions(pod.Name, nil)
				if err != nil {
					return 0, err
				}
				klog.Info("Configurations: ", confPredictions)
				confIndex = fmt.Sprintf("%dP_%s", len(tmpUuids), nodeModel)
				confPrediction, ok := confPredictions[confIndex]
				if !ok {
					var fbFree, util float32 = 100, 0
					if len(metrics) != 0 {
						util = metrics[uuid]["DCGM_FI_PROF_GR_ENGINE_ACTIVE"]
						fbFree = metrics[uuid]["DCGM_FI_DEV_FB_FREE"]
					}
					tmpScore := float64(fbFree-fbFree*util) * sum
					if int64(tmpScore) > score {
						score = int64(tmpScore)
						selectedUUID = uuid
					}
					continue
				}

				var interference float32 = 0
				tmpInterference, err := GetInterferencePredictions(pod.Name, nil)
				if err != nil {
					return 0, err
				}
				for collocatedPod := range SLOs[uuid] {
					for tmpPod, val := range tmpInterference {
						if strings.Contains(collocatedPod.Name, tmpPod) {
							interference += val
							break
						}
					}
				}
				klog.Info("Interference: ", tmpInterference)

				sum += 1 / (math.Abs(float64(float32(currSLO) - (confPrediction - interference))))

				var fbFree, util float32 = 100, 0
				if len(metrics) != 0 {
					util = metrics[uuid]["DCGM_FI_PROF_GR_ENGINE_ACTIVE"]
					fbFree = metrics[uuid]["DCGM_FI_DEV_FB_FREE"]
				}
				tmpScore := float64(fbFree-fbFree*util) * sum
				if int64(tmpScore) > score {
					score = int64(tmpScore)
					selectedUUID = uuid
				}
			}
		}
		podDesc, err := resources.New(pod.GetNamespace(), "", "", clientset)
		if err != nil {
			klog.Info("Pods.New() failed in PostBind: ", err.Error())
		}
		_, err = podDesc.AppendToExistingConfigMapsInPod(
			pod.GetName(),
			// Here find the optimal values for the env variables and replace them below
			map[string]string{
				nodeName: selectedUUID,
			},
		)
	}

	klog.Info("Score for node {", nodeName, "} = ", score)
	return int64(score), nil
}

func (g *GPU) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	clientset, err := utils.CheckClientset(clientset)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Error in utils.CheckClientset() in Score(): %s", err))
	}

	nodeInfo, err := g.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

	score, err := Logic(nodeInfo.Node().Name, pod, clientset)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Error in Logic() in Score(): %s", err))
	}

	return score, nil
}

func (g *GPU) ScoreExtensions() framework.ScoreExtensions {
	return g
}

func (g *GPU) NormalizeScore(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, scores framework.NodeScoreList) *framework.Status {
	// Find highest and lowest scores.
	var highest int64 = -math.MaxInt64
	var lowest int64 = math.MaxInt64
	for _, nodeScore := range scores {
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
	}

	// Transform the highest to lowest score range to fit the framework's min to max node score range.
	oldRange := highest - lowest
	newRange := framework.MaxNodeScore - framework.MinNodeScore
	for i, nodeScore := range scores {
		if oldRange == 0 {
			scores[i].Score = framework.MinNodeScore
		} else {
			scores[i].Score = ((nodeScore.Score - lowest) * newRange / oldRange) + framework.MinNodeScore
		}
	}

	return nil
}

func (g *GPU) PostBind(ctx context.Context, state *framework.CycleState, p *corev1.Pod, nodeName string) {
	var err error
	clientset, err = utils.CheckClientset(clientset)

	redisUrls, err := utils.FindNodesIPFromPod("-0", "redis", "", clientset, nil)
	if err != nil {
		klog.Info("FindNodesIP() failed in PostBind: ", err.Error())
	}
	if redisUrls == nil {
		klog.Fatal("Redis not found")
	}
	var redisUrl string
	for _, value := range redisUrls[0] {
		redisUrl = value
	}
	// Add redis port and password in k8s secret
	redisDesc := client.New(redisUrl+":32767", "1234", 0)

	if err != nil {
		klog.Fatal("Error in utils.CheckClientset() in PostBind: ", err)
	}

	val, err := redisDesc.Get(nodeName)
	if err != nil {
		klog.Fatal("Error occured in redis.Get() in PostBind: ", err)
	}
	var tmpUuids []string
	json.Unmarshal([]byte(val), &tmpUuids)
	isMig := utils.Exists(tmpUuids, "MIG")
	if err != nil {
		klog.Fatal("Error occured in utils.Exists() in PostBind: ", err)
	}

	countSave := 0
	if isMig != -1 {
		mu.Lock()
		count = count + 1
		countSave = count
		klog.Info("Count for pod {", p.GetName(), "} = ", countSave)

		if countSave > 5 {
			count = 0
			// Compute next partition here
			var nextPartition int
			for i, partition := range partitions {
				if len(tmpUuids) == partition {
					nextPartition = (i + 1) % len(partitions)
					break
				}
			}
			// Every five pods reconfigure A30
			if len(tmpUuids) != partitions[nextPartition] {
				params := resources.PatchNodeParam{
					Node:         nodeName,
					OperatorType: "replace",
					OperatorPath: "/metadata/labels",
					OperatorData: map[string]interface{}{
						"nvidia.com/mig.config": configs[nextPartition],
					},
				}
				_, err = params.LabelNode(clientset)
				if err != nil {
					klog.Fatal("Error occured in resources.PatchNode() in PostBind: ", err)
				}

				// Delete profiler pod to get updated with new UUIDs
				podsDesc, err := resources.New("redis", "", "", clientset)
				if err != nil {
					klog.Fatal("Error occured in resources.New() in PostBind: ", err)
				}
				podsList, err := podsDesc.ListPods()
				if err != nil {
					klog.Fatal("Error occured in resources.ListPods(): ", err)
				}
				var nodesProfiler v1.Pod
				for _, pod := range podsList.Items {
					if strings.Contains(pod.GetName(), "profiler") && pod.Spec.NodeName == nodeName {
						nodesProfiler = pod
						break
					}
				}
				klog.Info("nodesProfiler = ", nodesProfiler.GetName())
				var gracePeriodSeconds int64 = 0
				podsDesc.DeletePod(nodesProfiler.GetName(), metav1.DeleteOptions{GracePeriodSeconds: &gracePeriodSeconds})

				// Wait until reconfiguration is finished
				newUuids, err := redisDesc.Get(nodeName)
				if err != nil {
					klog.Fatal("Error occured in redis.Get() in PostBind: ", err)
				}
				for newUuids == val {
					time.Sleep(2 * time.Second)
					newUuids, err = redisDesc.Get(nodeName)
					if err != nil {
						klog.Fatal("Error occured in redis.Get() in PostBind: ", err)
					}
				}
			}
		}
		mu.Unlock()
	}

	var cfgMapName string = ""
	for _, envFrom := range p.Spec.Containers[0].EnvFrom {
		if envFrom.ConfigMapRef != nil {
			cfgMapName = envFrom.ConfigMapRef.LocalObjectReference.Name
		}
	}

	podDesc, err := resources.New(p.GetNamespace(), "", "", clientset)
	if err != nil {
		klog.Info("Pods.New() failed in PostBind: ", err.Error())
	}
	var cfgMap *v1.ConfigMap
	if cfgMapName != "" {
		cfgMap, err = podDesc.GetConfigMap(cfgMapName)
	}
	var uuid string = tmpUuids[rand.Intn(len(tmpUuids))]
	for key, value := range cfgMap.Data {
		if key == nodeName {
			uuid = value
			break
		}
	}

	node := BoundNode{Name: nodeName, UUID: uuid}
	gpuByteArray, err := json.Marshal(node)
	if err != nil {
		klog.Info("json.Marshal() failed in PostBind: ", err.Error())
	}

	collocatedPods, err := redisDesc.Get(string(gpuByteArray))
	if err != nil {
		klog.Info("error in redisDesc.Get(string(gpuByteArray)) in Logic(): ", err)
	}
	podList := []Pod{}
	if collocatedPods != "" {
		json.Unmarshal([]byte(collocatedPods), &podList)
	}
	podList = append(podList, Pod{Name: p.GetName(), Namespace: p.GetNamespace()})
	podListByteArray, err := json.Marshal(podList)
	if err != nil {
		klog.Info("json.Marshal() failed in PostBind: ", err.Error())
	}

	err = redisDesc.Set(string(gpuByteArray), string(podListByteArray))
	if err != nil {
		klog.Info("Redis Set() failed in PostBind: ", err.Error())
	}

	// Add CUDA_VISIBLE_DEVICES in the ConfigMap so that it gets into pod's env
	_, err = podDesc.AppendToExistingConfigMapsInPod(
		p.GetName(),
		// Here find the optimal values for the env variables and replace them below
		map[string]string{
			"CUDA_VISIBLE_DEVICES": uuid,
			// "CUDA_MPS_PINNED_DEVICE_MEM_LIMIT":  "0=16350MB",
			// "CUDA_MPS_ACTIVE_THREAD_PERCENTAGE": "50",
		},
	)
	if err != nil {
		klog.Info("AppendToExistingConfigMapsInPod() failed in PostBind: ", err.Error())
	}
}

func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &GPU{handle: h}, nil
}
