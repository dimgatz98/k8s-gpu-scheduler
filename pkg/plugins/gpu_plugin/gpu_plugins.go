package podstate

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"k8s-gpu-scheduler/pkg/pods"
	promMetrics "k8s-gpu-scheduler/pkg/prom/fetch_prom_metrics"
	"k8s-gpu-scheduler/pkg/redis/client"
	"k8s-gpu-scheduler/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

const Name = "GPU"
const ConfigMapName = "gpu-scheduler-cfgmap"

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
	nodeName := nodeInfo.Node().GetName()
	klog.Info("Scoring node: ", nodeName)

	dcgmPod, err := utils.GetNodesDcgmPod(nodeName)
	utils.Check(err)

	// Get prometheus node's IP
	promUrl, err := utils.FindNodesIP("prometheus-0", "", "")
	utils.Check(err)

	// Fetch DCGM data and score node based on the data
	responses, err := promMetrics.DcgmPromInstantQuery("http://"+promUrl+":30090/", "{pod=\""+dcgmPod+"\"}")
	if responses == nil {
		return 0, nil
	}
	utils.Check(err)
	minUtil, maxFbFree := math.MaxInt, math.MinInt
	// Fix this to take the same partition's metrics instead of the fittest for each metric
	for _, response := range responses {
		if strings.Contains(response.MetricName, "DCGM_FI_PROF_GR_ENGINE_ACTIVE") {
			value, err := strconv.Atoi(response.Value)
			utils.Check(err)
			if value < minUtil {
				minUtil = value
			}
		} else if strings.Contains(response.MetricName, "DCGM_FI_DEV_FB_FREE") {
			value, err := strconv.Atoi(response.Value)
			utils.Check(err)
			if value > maxFbFree {
				maxFbFree = value
			}
		}
		klog.Info(response.MetricName, " for node {", nodeName, "} = ", response.Value)
	}

	score := 0
	score += maxFbFree - maxFbFree*minUtil

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

	redisUrl, err := utils.FindNodesIP("-0", "redis", "")
	if err != nil {
		klog.Info("FindNodesIP() failed in PostBind: ", err.Error())
	}
	// Add redis port and password in k8s secret or dotenv
	desc := client.New(redisUrl+":32767", "1234", 0)
	// Find node's uuids
	data, err := desc.Get(nodeName)
	if err != nil {
		klog.Info("Redis Get() failed in PostBind: ", err.Error())
	}

	var uuids []string
	json.Unmarshal([]byte(data), &uuids)
	uuid := uuids[0]
	// // Score uuids and choose the fittest
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
	err = desc.Set(string(podByteArray), string(nodeByteArray))
	if err != nil {
		klog.Info("Redis Set() failed in PostBind: ", err.Error())
	}

	// Add CUDA_VISIBLE_DEVICES in the ConfigMap so that it get into pod's env
	podDesc, err := pods.New(p.GetNamespace(), "", "")
	if err != nil {
		klog.Info("Pods.New() failed in PostBind: ", err.Error())
	}
	mu.Lock()
	defer mu.Unlock()
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
	count++
}

func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &GPU{handle: h}, nil
}
