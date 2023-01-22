package utils

import (
	"context"
	"k8s-gpu-scheduler/pkg/pods"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func FindNodesIP(podNameContains string, namespace string, fieldSelector string) (string, error) {
	desc, err := pods.New(namespace, fieldSelector, "")
	if err != nil {
		return "", err
	}
	podsList, err := desc.ListPods()
	if err != nil {
		return "", err
	}

	var url string
	for _, pod := range podsList.Items {
		if strings.Contains(pod.GetName(), podNameContains) {
			promNode, err := desc.Clientset.CoreV1().Nodes().Get(
				context.TODO(),
				pod.Spec.NodeName,
				metav1.GetOptions{},
			)
			if err != nil {
				return "", err
			}
			url = promNode.Status.Addresses[0].Address
			break
		}
	}
	return url, nil
}

func GetNodesDcgmPod(nodeName string) (string, error) {
	desc, err := pods.New("", "spec.nodeName="+nodeName, "")
	if err != nil {
		return "", nil
	}
	podsList, err := desc.ListPods()
	if err != nil {
		return "", nil
	}
	var dcgmPod string
	for _, pod := range podsList.Items {
		if strings.Contains(pod.GetName(), "dcgm") {
			dcgmPod = pod.GetName()
			break
		}
	}
	klog.Info("Dcgm pod for node ", nodeName, " = ", dcgmPod)
	return dcgmPod, nil
}
