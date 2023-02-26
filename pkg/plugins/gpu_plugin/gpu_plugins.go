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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
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

type Listers struct {
	configMapLister v1.ConfigMapLister
	podsLister      v1.PodLister
}

type Indexers struct {
	configMapIndexer cache.Indexer
	podIndexer       cache.Indexer
}

type Pod struct {
	Name      string
	Namespace string
}

type BoundNode struct {
	Name string
	UUID string
}

var factory informers.SharedInformerFactory = nil
var listers Listers
var indexers Indexers

func (g *GPU) Name() string {
	return Name
}

func GetSLOs(nodeName string, uuids []string, clientset *kubernetes.Clientset, redisDesc *client.Descriptor) (res map[string]map[Pod]float32, err error) {
	res = map[string]map[Pod]float32{}

	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeName)
	podsDesc, err := resources.New("", fieldSelector, "", clientset)
	if err != nil {
		return nil, err
	}
	podsList, err := podsDesc.ListPods(listers.podsLister)
	if err != nil {
		return nil, err
	}

	uuidsSet := map[string]int{}
	for i, uuid := range uuids {
		uuidsSet[uuid] = i
	}
	for _, pod := range podsList {
		// If PodSucceeded, PodFailed or PodUnknown ignore
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
			continue
		}

		uuid := ""
		found := false
		for _, envFrom := range pod.Spec.Containers[0].EnvFrom {
			if envFrom.ConfigMapRef == nil {
				continue
			}
			cfgMapName := envFrom.ConfigMapRef.Name
			cfgMap, err := podsDesc.GetConfigMap(cfgMapName, indexers.configMapIndexer)
			if err != nil {
				continue
			}
			for key, val := range cfgMap.Data {
				if key == "CUDA_VISIBLE_DEVICES" {
					uuid = val
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if uuid == "" {
			continue
		}

		_, ok := uuidsSet[uuid]
		if ok {
			value := utils.GetEnv(pod, "SLO")
			if value == "" {
				continue
			}
			floatSLO, err := strconv.ParseFloat(value, 32)
			if err != nil {
				return nil, err
			}
			_, ok := res[uuid]
			if ok {
				res[uuid][Pod{Name: pod.Name, Namespace: pod.Namespace}] = float32(floatSLO)
			} else {
				res[uuid] = map[Pod]float32{
					{
						Name:      pod.Name,
						Namespace: pod.Namespace,
					}: float32(floatSLO)}
			}

		}
	}

	return res, nil
}

func GetDcgmMetricsForUUIDS(nodeName string, clientset *kubernetes.Clientset, podList []*corev1.Pod, uuids []string) (metrics map[string]map[string]float32, err error) {
	dcgmPod, err := utils.GetNodesDcgmPod(listers.podsLister, nodeName, podList, clientset)
	if dcgmPod == "" {
		return nil, nil
	}
	log.Println("dcgmPod: ", dcgmPod)
	utils.Check(err)

	// Get prometheus node's IP
	promUrls, err := utils.FindNodesIPFromPod(listers.podsLister, "prometheus-0", "prometheus", "", clientset, nil)
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
		return gpuIds[i] < gpuIds[j]
	})
	gpuIdToUUID := map[string]string{}
	count := -1
	for _, gpuId := range gpuIds {
		_, ok := gpuIdToUUID[gpuId]
		if ok {
			gpuIdToUUID[gpuId] = uuids[count]
			continue
		}
		count++
		gpuIdToUUID[gpuId] = uuids[count]
	}
	klog.Info("gpuIds: ", gpuIds, "\ngpuIdToUUID: ", gpuIdToUUID)

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

func GetDcgmMetricsForNode(nodeName string, clientset *kubernetes.Clientset, podList []*corev1.Pod) (metrics map[string]map[string]float32, err error) {
	if podList == nil {
		desc, err := resources.New("", "spec.nodeName="+nodeName, "", clientset)
		if err != nil {
			return nil, err
		}
		podList, err = desc.ListPods(listers.podsLister)
		if err != nil {
			return nil, err
		}
	}

	dcgmPod, err := utils.GetNodesDcgmPod(listers.podsLister, nodeName, podList, clientset)
	if dcgmPod == "" {
		return nil, nil
	}
	log.Println("dcgmPod: ", dcgmPod)
	utils.Check(err)

	// Get prometheus node's IP
	promUrls, err := utils.FindNodesIPFromPod(listers.podsLister, "prometheus-0", "", "", clientset, podList)
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

func GetConfigurationPredictions(podName string, podList []*corev1.Pod) (configurations map[string]float32, err error) {
	// Find recommender node's IP
	recommenderIPs, err := utils.FindNodesIPFromPod(listers.podsLister, "recommender", "recommender", "", clientset, podList)
	klog.Info("recommenderIPs: ", recommenderIPs)
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

func GetInterferencePredictions(podName string, podList []*corev1.Pod) (interference map[string]float32, err error) {
	// Find recommender node's IP
	recommenderIPs, err := utils.FindNodesIPFromPod(listers.podsLister, "recommender", "recommender", "", clientset, podList)
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

func Logic(nodeName string, pod *corev1.Pod, clientset *kubernetes.Clientset) (int64, error) {
	var err error
	var score int64 = framework.MinNodeScore
	var selectedUUID string = ""
	var currSLO float32

	tmpSLO := utils.GetEnv(pod, "SLO")
	if tmpSLO == "" {
		currSLO = 0
	} else {
		tmp, err := strconv.ParseFloat(tmpSLO, 32)
		currSLO = float32(tmp)
		if err != nil {
			currSLO = 0
		}
	}

	redisUrls, err := utils.FindNodesIPFromPod(listers.podsLister, "-0", "redis", "", clientset, nil)
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
			// fbFree := metrics[key]["DCGM_FI_DEV_FB_FREE"]
			tmpScore := 100 * (1 - util)
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
			k := 0.5
			for _, uuid := range tmpUuids {
				var negative_sum float64 = 0
				var positive_sum float64 = 0
				count_negative := 0
				count_positive := 0

				_, ok := SLOs[uuid]
				// If the key exists
				if ok {
					for scheduledPod, SLO := range SLOs[uuid] {
						if SLO == 0 {
							continue
						}

						confIndex := ""
						confPredictions, err := GetConfigurationPredictions(scheduledPod.Name, nil)
						if err != nil {
							return 0, err
						}

						confIndex = fmt.Sprintf("%dP_%s", len(tmpUuids), nodeModel)
						confPrediction, ok := confPredictions[confIndex]
						if !ok {
							continue
						}

						var interference float32 = 0
						tmpInterference, err := GetInterferencePredictions(scheduledPod.Name, nil)
						if err != nil {
							return 0, err
						}
						for collocatedPod := range SLOs[uuid] {
							if scheduledPod == collocatedPod {
								continue
							}

							for tmpPod, val := range tmpInterference {
								if strings.Contains(strings.ReplaceAll(collocatedPod.Name, "-", "_"), tmpPod) {
									interference += val
									break
								}
							}
						}

						for tmpPod, val := range tmpInterference {
							if strings.Contains(strings.ReplaceAll(pod.Name, "-", "_"), tmpPod) {
								interference += val
								break
							}
						}

						klog.Infof("for pod %s SLO: %f, configuration: %f, interference: %f", scheduledPod.Name, SLO, confPrediction, interference)
						// find SLO > (confPrediction-interference) and multiply by k
						if SLO > (confPrediction - interference) {
							count_negative += 1
							negative_sum += 1 / (1 + math.Pow(math.Abs(float64((1/SLO)*(SLO-(confPrediction-interference))))+1, 2))
						} else {
							count_positive += 1
							positive_sum += 1 / (1 + math.Abs(float64((1/SLO)*(SLO-(confPrediction-interference)))))
						}
					}
				}

				// For the pod that is being scheduled
				confIndex := ""
				confPredictions, err := GetConfigurationPredictions(pod.Name, nil)
				if err != nil {
					return 0, err
				}

				confIndex = fmt.Sprintf("%dP_%s", len(tmpUuids), nodeModel)
				confPrediction, ok := confPredictions[confIndex]
				if !ok {
					var util float32 = 0
					// var fbFree float32 = 1
					if len(metrics) != 0 {
						util = metrics[uuid]["DCGM_FI_PROF_GR_ENGINE_ACTIVE"]
						// fbFree = metrics[uuid]["DCGM_FI_DEV_FB_FREE"]
					}

					factor := 100 * (float64(1 - util))
					var tmpScore float64 = 0

					if count_positive > 0 && count_negative > 0 {
						tmpScore = factor*((1-k)*positive_sum/float64(count_positive)) + factor*(k*negative_sum/float64(count_negative))
					} else if count_negative > 0 {
						tmpScore = factor * (negative_sum / float64(count_negative))
					} else if count_positive > 0 {
						tmpScore = factor * (positive_sum / float64(count_positive))
					}

					klog.Infof("tmpScore for uuid %s: = %f", uuid, tmpScore)
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
						if strings.Contains(strings.ReplaceAll(collocatedPod.Name, "-", "_"), tmpPod) {
							interference += val
							break
						}
					}
				}
				klog.Info("Interference: ", tmpInterference)

				if currSLO > (confPrediction - interference) {
					count_negative += 1
					negative_sum += 1 / (1 + math.Pow(math.Abs(float64((1/currSLO)*(currSLO-(confPrediction-interference))))+1, 2))
				} else {
					count_positive += 1
					positive_sum += 1 / (1 + math.Abs(float64((1/currSLO)*(currSLO-(confPrediction-interference)))))
				}

				var fbFree, util float32 = 1, 0
				if len(metrics) != 0 {
					util = metrics[uuid]["DCGM_FI_PROF_GR_ENGINE_ACTIVE"]
					fbFree = metrics[uuid]["DCGM_FI_DEV_FB_FREE"]
				}

				factor := 100 * (float64(1 - util))
				var tmpScore float64 = 0

				if count_positive > 0 && count_negative > 0 {
					tmpScore = factor*((1-k)*positive_sum/float64(count_positive)) + factor*(k*negative_sum/float64(count_negative))
				} else if count_negative > 0 {
					tmpScore = factor * (negative_sum / float64(count_negative))
				} else if count_positive > 0 {
					tmpScore = factor * (positive_sum / float64(count_positive))
				}

				klog.Infof(
					"Metrics for UUID %s: fb = %f, util = %f, interference = %f, configuration = %f",
					uuid,
					fbFree,
					util,
					interference,
					confPrediction,
				)
				klog.Infof("tmpScore for pod %s and uuid %s: = %f", pod.Name, uuid, tmpScore)
				if int64(tmpScore) > score {
					score = int64(tmpScore)
					selectedUUID = uuid
				}
			}
		}

		klog.Info("selectedUUID: ", selectedUUID)
		podDesc, err := resources.New(pod.GetNamespace(), "", "", clientset)
		if err != nil {
			klog.Info("Pods.New() failed in PostBind: ", err.Error())
		}
		err = podDesc.AppendToExistingConfigMapsInPod(
			indexers.podIndexer,
			indexers.configMapIndexer,
			pod.GetName(),
			map[string]string{
				nodeName: selectedUUID,
			},
			true,
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

	if factory == nil {
		factory = informers.NewSharedInformerFactory(clientset, 10*time.Minute)
		listers.configMapLister = factory.Core().V1().ConfigMaps().Lister()
		listers.podsLister = factory.Core().V1().Pods().Lister()
		indexers.configMapIndexer = factory.Core().V1().ConfigMaps().Informer().GetIndexer()
		indexers.podIndexer = factory.Core().V1().Pods().Informer().GetIndexer()
		stopCh := make(chan struct{})
		factory.Start(stopCh)
		factory.WaitForCacheSync(stopCh)
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

	redisUrls, err := utils.FindNodesIPFromPod(listers.podsLister, "-0", "redis", "", clientset, nil)
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

	var cfgMapName string = ""
	for _, envFrom := range p.Spec.Containers[0].EnvFrom {
		if envFrom.ConfigMapRef != nil {
			cfgMapName = envFrom.ConfigMapRef.LocalObjectReference.Name
			break
		}
	}

	podDesc, err := resources.New(p.GetNamespace(), "", "", clientset)
	if err != nil {
		klog.Info("Pods.New() failed in PostBind: ", err.Error())
	}
	var cfgMap *corev1.ConfigMap
	if cfgMapName != "" {
		cfgMap, err = podDesc.GetConfigMap(cfgMapName, indexers.configMapIndexer)
	}
	klog.Info("cfgMapName: ", cfgMapName)

	if cfgMap != nil {
		var uuid string = tmpUuids[rand.Intn(len(tmpUuids))]
		for key, value := range cfgMap.Data {
			if key == nodeName {
				uuid = value
				break
			}
		}

		klog.Info("UUID: ", uuid)
		// Add CUDA_VISIBLE_DEVICES in the ConfigMap so that it gets into pod's env
		if uuid != "" {
			err = podDesc.AppendToExistingConfigMapsInPod(
				indexers.podIndexer,
				indexers.configMapIndexer,
				p.GetName(),
				// Here find the optimal values for the env variables and replace them below
				map[string]string{
					"CUDA_VISIBLE_DEVICES": uuid,
					// "CUDA_MPS_PINNED_DEVICE_MEM_LIMIT":  "0=16350MB",
					// "CUDA_MPS_ACTIVE_THREAD_PERCENTAGE": "50",
				},
				true,
			)
			if err != nil {
				klog.Info("AppendToExistingConfigMapsInPod() failed in PostBind: ", err.Error())
			}
		}
	}
}

func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &GPU{handle: h}, nil
}
