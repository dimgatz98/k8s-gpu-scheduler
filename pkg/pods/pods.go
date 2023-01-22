package pods

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type PatchStringValue struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

type PatchPodParam struct {
	Pod          string                 `json:"pod"`
	OperatorType string                 `json:"operator_type"`
	OperatorPath string                 `json:"operator_path"`
	OperatorData map[string]interface{} `json:"operator_data"`
}

type (
	Descriptor struct {
		Clientset     *kubernetes.Clientset
		Namespace     string
		FieldSelector string
	}

	Resources interface {
		ListPods()
		Patch(params *PatchPodParam)
		AddEnv(podName string, env map[string]string) (result *v1.Pod, err error)
		Get(podName string) (result *v1.Pod, err error)
		UpdateConfigMap(mapName string, data map[string]string) (ret *v1.ConfigMap, err error)
		CreateConfigMap(mapName string, data map[string]string) (ret *v1.ConfigMap, err error)
		AppendToExistingConfigMapsInPod(podName string, data map[string]string) (ret []*v1.ConfigMap, err error)
	}
)

func (r *Descriptor) ListPods() (*corev1.PodList, error) {
	list, err := r.Clientset.CoreV1().Pods(r.Namespace).List(
		context.TODO(),
		metav1.ListOptions{
			FieldSelector: r.FieldSelector,
		})
	return list, err
}

func (r *Descriptor) Patch(params *PatchPodParam) (result *v1.Pod, err error) {
	coreV1 := r.Clientset.CoreV1()

	operatorData := params.OperatorData
	operatorType := params.OperatorType
	operatorPath := params.OperatorPath

	var payloads []interface{}

	for key, value := range operatorData {
		payload := PatchStringValue{
			Op:    operatorType,
			Path:  operatorPath + key,
			Value: value,
		}

		payloads = append(payloads, payload)
	}
	payloadBytes, _ := json.Marshal(payloads)

	result, err = coreV1.Pods(r.Namespace).Patch(context.TODO(), params.Pod, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	return result, err
}

func (r *Descriptor) Get(podName string) (result *v1.Pod, err error) {
	coreV1 := r.Clientset.CoreV1()

	pod, err := coreV1.Pods(r.Namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	return pod, err
}

func (r *Descriptor) UpdateConfigMap(cfgMapName string, data map[string]string) (ret *v1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()

	cfgMap, err := coreV1.ConfigMaps(r.Namespace).Get(context.Background(), cfgMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		cfgMap.Data[key] = value
	}
	ret, err = coreV1.ConfigMaps(r.Namespace).Update(context.Background(), cfgMap, metav1.UpdateOptions{})
	return ret, err
}

func (r *Descriptor) CreateConfigMap(mapName string, data map[string]string) (ret *v1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()

	cfgMap := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      mapName,
			Namespace: r.Namespace,
		},
		Data: data,
	}

	for key, value := range data {
		cfgMap.Data[key] = value
	}
	ret, err = coreV1.ConfigMaps(r.Namespace).Create(context.Background(), &cfgMap, metav1.CreateOptions{})
	return ret, err
}

func (r *Descriptor) GetConfigMap(cfgMapName string) (ret *v1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()
	ret, err = coreV1.ConfigMaps(r.Namespace).Get(context.Background(), cfgMapName, metav1.GetOptions{})
	return ret, err
}

func (r *Descriptor) AppendToExistingConfigMapsInPod(podName string, data map[string]string) (ret []*v1.ConfigMap, err error) {
	pod, err := r.Get(podName)
	if err != nil {
		return nil, err
	}

	var tmpData map[string]string
	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				cfgMapName := envFrom.ConfigMapRef.LocalObjectReference.Name
				if err != nil {
					return nil, err
				}
				tmpData = data
				// Check if keys already exist and if so ignore them
				cfgMap, err := r.GetConfigMap(cfgMapName)
				for key := range data {
					if KeyExists(key, cfgMap.Data) {
						delete(tmpData, key)
					}
				}
				tmpRet, err := r.UpdateConfigMap(cfgMapName, tmpData)
				ret = append(ret, tmpRet)
				if err != nil {
					return ret, err
				}
			}
		}
	}

	return ret, err
}

func KeyExists(key string, m map[string]string) bool {
	for k := range m {
		if key == k {
			return true
		}
	}
	return false
}

func New(namespace string, fieldSelector string, configPath string) (r *Descriptor, err error) {
	var config *rest.Config
	if configPath == "" {
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", "/home/dimitris/.kube/config")
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Descriptor{
		Clientset:     clientset,
		Namespace:     namespace,
		FieldSelector: fieldSelector,
	}, nil
}
