package utils

import (
	"strings"

	recommender "github.com/dimgatz98/k8s-gpu-scheduler/pkg/recommender/go_client"
)

func FindMaxIndForNode(nodeName string, reply *recommender.Reply) float32 {
	var res float32 = 0
	for i, col := range reply.Columns {
		if strings.Contains(col, nodeName) && res < reply.Result[i] {
			res = reply.Result[i]
		}
	}

	return res
}
