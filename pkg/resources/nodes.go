package resources

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type (
	PatchNodeParam struct {
		Node         string                 `json:"node"`
		OperatorType string                 `json:"operator_type"`
		OperatorPath string                 `json:"operator_path"`
		OperatorData map[string]interface{} `json:"operator_data"`
	}

	Node interface {
		PatchNode(params *PatchNodeParam)
	}
)

func GetNode(indexer cache.Indexer, nodeName string) (*corev1.Node, error) {
	tmp, exists, err := indexer.GetByKey("k8s-aferik-master")
	if err != nil {
		return nil, err
	}
	if !exists {
		return &corev1.Node{}, err
	}
	return tmp.(*corev1.Node), err
}

func (p *PatchNodeParam) LabelNode(indexer cache.Indexer, clientset *kubernetes.Clientset) (result *v1.Node, err error) {
	coreV1 := clientset.CoreV1()
	node := p.Node

	data, err := GetNode(indexer, p.Node)
	operatorData := data.Labels
	if err != nil {
		return nil, err
	}
	for key, value := range p.OperatorData {
		operatorData[key] = value.(string)
	}
	operatorType := p.OperatorType
	operatorPath := p.OperatorPath

	payload := PatchStringValue{
		Op:    operatorType,
		Path:  operatorPath,
		Value: operatorData,
	}
	payloads := []PatchStringValue{payload}
	payloadBytes, _ := json.Marshal(payloads)

	result, err = coreV1.Nodes().Patch(context.TODO(), node, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, err
	}

	return result, err
}
