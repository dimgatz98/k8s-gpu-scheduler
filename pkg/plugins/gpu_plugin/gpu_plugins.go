package podstate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimgatz98/k8s-gpu-scheduler/utils"

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

func (g *GPU) Score(ctx context.Context, state *framework.CycleState, pod *corev1.Pod, nodeName string) (int64, *framework.Status) {
	nodeInfo, err := g.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

	return g.score(nodeInfo)
}

func (g *GPU) ScoreExtensions() framework.ScoreExtensions {
	return g
}

func (g *GPU) score(nodeInfo *framework.NodeInfo) (int64, *framework.Status) {
	var err error
	clientset, err = utils.CheckClientset(clientset)
	if err != nil {
		klog.Fatal("Error in utils.CheckClientset() in score: ", err)
	}

	nodeName := nodeInfo.Node().GetName()
	klog.Info("Scoring node: ", nodeName)

	dcgmPod, err := utils.GetNodesDcgmPod(nodeName, clientset)
	utils.Check(err)

	// Get prometheus node's IP
	promUrls, err := utils.FindNodesIPFromPod("prometheus-0", "", "", clientset)
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
	responses, err := promMetrics.DcgmPromInstantQuery("http://"+promUrl+":30090/", "{pod=\""+dcgmPod+"\"}")
	if responses == nil {
		return 0, nil
	}
	utils.Check(err)
	metrics := make(map[string]map[string]float32)
	for _, response := range responses {
		var key string
		if response.GPU_I_ID != "" {
			key = response.GPU_I_ID
		} else {
			key = "NOT_MIG"
		}
		value, err := strconv.ParseFloat(response.Value, 32)
		utils.Check(err)
		if _, found := metrics[key]; !found {
			metrics[key] = map[string]float32{}
		}
		metrics[key][response.MetricName] = float32(value)
		klog.Info(response.MetricName, " for node {", nodeName, "} = ", response.Value)
	}

	score := framework.MinNodeScore
	for _, value := range metrics {
		var util, fbFree float32 = 1, 0
		for metricName, metric := range value {
			if metricName == "DCGM_FI_PROF_GR_ENGINE_ACTIVE" {
				util = metric
			} else if metricName == "DCGM_FI_DEV_FB_FREE" {
				fbFree = metric
			}
		}
		tmpScore := int64(fbFree - fbFree*util)
		if tmpScore > score {
			score = tmpScore
		}
	}

	klog.Info("Score for node {", nodeName, "} = ", score)
	return int64(score), nil
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

	redisUrls, err := utils.FindNodesIPFromPod("-0", "redis", "", clientset)
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

	// Find node's uuids
	data, err := redisDesc.Get(nodeName)
	if err != nil {
		klog.Info("Redis Get() failed in PostBind: ", err.Error())
	}

	var uuids []string
	json.Unmarshal([]byte(data), &uuids)
	uuid := uuids[countSave%len(uuids)]

	klog.Info("Selected UUID: ", uuid)
	// If GPU is MIG enabled score uuids and choose the fittest partition
	// for _, uuid := range uuids {
	// 		pass
	// }

	pod := Pod{Name: p.GetName(), Namespace: p.GetNamespace()}
	node := BoundNode{Name: nodeName, UUID: uuid}
	podByteArray, err := json.Marshal(pod)
	if err != nil {
		klog.Info("json.Marshal() failed in PostBind: ", err.Error())
	}
	nodeByteArray, err := json.Marshal(node)
	if err != nil {
		klog.Info("json.Marshal() failed in PostBind: ", err.Error())
	}
	err = redisDesc.Set(string(podByteArray), string(nodeByteArray))
	if err != nil {
		klog.Info("Redis Set() failed in PostBind: ", err.Error())
	}

	// Add CUDA_VISIBLE_DEVICES in the ConfigMap so that it get into pod's env
	podDesc, err := resources.New(p.GetNamespace(), "", "", clientset)
	if err != nil {
		klog.Info("Pods.New() failed in PostBind: ", err.Error())
	}
	_, err = podDesc.AppendToExistingConfigMapsInPod(
		p.GetName(),
		// Here find the optimal values for the env variables and replace them below
		map[string]string{
			"CUDA_VISIBLE_DEVICES":              uuid,
			"CUDA_MPS_PINNED_DEVICE_MEM_LIMIT":  "0=16350MB",
			"CUDA_MPS_ACTIVE_THREAD_PERCENTAGE": "50",
		},
	)
	if err != nil {
		klog.Info("AddToExistingConfigMapsInPod() failed in PostBind: ", err.Error())
	}
}

func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &GPU{handle: h}, nil
}
