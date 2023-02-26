package resources

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	v1 "k8s.io/client-go/listers/core/v1"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
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
		indexer       cache.Indexer
		Namespace     string
		FieldSelector string
	}

	Resources interface {
		ListPods()
		PatchPod(params *PatchPodParam)
		Get(podName string) (result *corev1.Pod, err error)
		UpdateConfigMap(mapName string, data map[string]string) (ret *corev1.ConfigMap, err error)
		CreateConfigMap(mapName string, data map[string]string) (ret *corev1.ConfigMap, err error)
		AppendToExistingConfigMapsInPod(podName string, data map[string]string) (ret []*corev1.ConfigMap, err error)
		DeletePod(podName string, options metav1.DeleteOptions) error
	}
)

var ctx = context.Background()

func (r *Descriptor) ListPods(podsLister v1.PodLister) (list []*corev1.Pod, err error) {
	selector := labels.NewSelector()
	list, err = podsLister.Pods(r.Namespace).List(selector)
	if err != nil {
		panic(err)
	}
	return list, err
}

func (r *Descriptor) PatchPod(params *PatchPodParam) (result *corev1.Pod, err error) {
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

	result, err = coreV1.Pods(r.Namespace).Patch(ctx, params.Pod, types.JSONPatchType, payloadBytes, metav1.PatchOptions{})
	return result, err
}

func (r *Descriptor) Get(podName string, indexer cache.Indexer) (result *corev1.Pod, err error) {
	tmp, exists, err := indexer.GetByKey(fmt.Sprintf("%s/%s", r.Namespace, podName))
	if err != nil {
		return nil, err
	}
	if !exists {
		return &corev1.Pod{}, nil
	}
	return tmp.(*corev1.Pod), err
}

func (r *Descriptor) UpdateConfigMap(indexer cache.Indexer, cfgMapName string, data map[string]string, overwrite bool) (ret *corev1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()

	cfgMap, err := r.GetConfigMap(cfgMapName, indexer)
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		_, ok := cfgMap.Data[key]
		if ok {
			if overwrite {
				cfgMap.Data[key] = value
			}
		} else {
			cfgMap.Data[key] = value
		}
	}
	ret, err = coreV1.ConfigMaps(r.Namespace).Update(ctx, cfgMap, metav1.UpdateOptions{})
	return ret, err
}

func (r *Descriptor) CreateConfigMap(mapName string, data map[string]string) (ret *corev1.ConfigMap, err error) {
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
	ret, err = coreV1.ConfigMaps(r.Namespace).Create(ctx, &cfgMap, metav1.CreateOptions{})
	return ret, err
}

func (r *Descriptor) GetConfigMap(cfgMapName string, indexer cache.Indexer) (ret *corev1.ConfigMap, err error) {
	tmp, exists, err := indexer.GetByKey(fmt.Sprintf("%s/%s", r.Namespace, cfgMapName))
	if err != nil {
		return nil, err
	}
	if !exists {
		return &corev1.ConfigMap{}, nil
	}
	return tmp.(*corev1.ConfigMap), err
}

func (r *Descriptor) AppendToExistingConfigMapsInPod(indexer cache.Indexer, podName string, data map[string]string, overwrite bool) (err error) {
	pod, err := r.Get(podName, indexer)
	if err != nil {
		return err
	}

	for _, container := range pod.Spec.Containers {
		for _, envFrom := range container.EnvFrom {
			if envFrom.ConfigMapRef != nil {
				cfgMapName := envFrom.ConfigMapRef.LocalObjectReference.Name
				_, err := r.UpdateConfigMap(indexer, cfgMapName, data, overwrite)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *Descriptor) DeletePod(podName string, options metav1.DeleteOptions) error {
	coreV1 := r.Clientset.CoreV1()
	err := coreV1.Pods(r.Namespace).Delete(ctx, podName, options)
	return err
}

func New(namespace string, fieldSelector string, configPath string, clientset *kubernetes.Clientset) (r *Descriptor, err error) {
	if namespace == "" {
		namespace = "default"
	}

	if clientset == nil {
		var config *rest.Config
		if configPath == "" {
			config, err = rest.InClusterConfig()
			if err != nil {
				return nil, err
			}
		} else {
			config, err = clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				return nil, err
			}
		}

		clientset, err = kubernetes.NewForConfig(config)
		if err != nil {
			return nil, err
		}
	}

	return &Descriptor{
		Clientset:     clientset,
		Namespace:     namespace,
		FieldSelector: fieldSelector,
	}, nil
}
