package utils

import (
	"context"
	"strings"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/resources"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func FindNodeFromPod(podNameContains string, namespace string, fieldSelector string, clientset *kubernetes.Clientset, podList *v1.PodList) ([]*v1.Node, error) {
	desc, err := resources.New(namespace, fieldSelector, "", clientset)
	if err != nil {
		return nil, err
	}

	if podList == nil {

		podList, err = desc.ListPods()
		if err != nil {
			return nil, err
		}
	}

	nodes := []*v1.Node{}
	for _, pod := range podList.Items {
		if strings.Contains(pod.GetName(), podNameContains) {
			node, err := desc.Clientset.CoreV1().Nodes().Get(
				context.TODO(),
				pod.Spec.NodeName,
				metav1.GetOptions{},
			)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}

func FindNodesIPFromPod(podNameContains string, namespace string, fieldSelector string, clientset *kubernetes.Clientset, podList *v1.PodList) ([]map[string]string, error) {
	nodes, err := FindNodeFromPod(podNameContains, namespace, fieldSelector, clientset, podList)
	if err != nil {
		return nil, err
	}
	var urls []map[string]string
	for _, node := range nodes {
		urls = append(urls, map[string]string{node.Name: node.Status.Addresses[0].Address})
	}

	return urls, nil
}

func GetNodesDcgmPod(nodeName string, podsList *v1.PodList, clientset *kubernetes.Clientset) (string, error) {

	if podsList == nil {
		desc, err := resources.New("", "spec.nodeName="+nodeName, "", clientset)
		if err != nil {
			return "", err
		}

		podsList, err = desc.ListPods()
		if err != nil {
			return "", err
		}
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

func Remove(slice []string, index int) []string {
	return append(slice[:index], slice[index+1:]...)
}

func Exists(slice []string, s string) int {
	for i, el := range slice {
		if strings.Contains(el, s) {
			return i
		}
	}
	return -1
}

func CheckClientset(clientset *kubernetes.Clientset) (*kubernetes.Clientset, error) {
	if clientset == nil {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
	}
	return clientset, nil
}

func GetEnv(pod *corev1.Pod, name string) (value string) {
	for _, envVar := range pod.Spec.Containers[0].Env {
		if envVar.Name == name {
			value = envVar.Value
			break
		}
	}
	return value
}
