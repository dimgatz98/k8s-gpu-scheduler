package resources

import (
	"context"
	"encoding/json"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
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

func GetLabels(nodeName string, clientset *kubernetes.Clientset) (map[string]string, error) {
	coreV1 := clientset.CoreV1()
	node, err := coreV1.Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	return node.Labels, err
}

func (p *PatchNodeParam) LabelNode(clientset *kubernetes.Clientset) (result *v1.Node, err error) {
	coreV1 := clientset.CoreV1()
	node := p.Node

	operatorData, err := GetLabels(p.Node, clientset)
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
