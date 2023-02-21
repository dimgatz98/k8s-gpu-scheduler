package resources

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
		PatchPod(params *PatchPodParam)
		Get(podName string) (result *v1.Pod, err error)
		UpdateConfigMap(mapName string, data map[string]string) (ret *v1.ConfigMap, err error)
		CreateConfigMap(mapName string, data map[string]string) (ret *v1.ConfigMap, err error)
		AppendToExistingConfigMapsInPod(podName string, data map[string]string) (ret []*v1.ConfigMap, err error)
		DeletePod(podName string, options metav1.DeleteOptions) error
	}
)

var ctx = context.Background()

func (r *Descriptor) ListPods() (*corev1.PodList, error) {
	list, err := r.Clientset.CoreV1().Pods(r.Namespace).List(
		ctx,
		metav1.ListOptions{
			FieldSelector: r.FieldSelector,
		})
	return list, err
}

func (r *Descriptor) PatchPod(params *PatchPodParam) (result *v1.Pod, err error) {
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

func (r *Descriptor) Get(podName string) (result *v1.Pod, err error) {
	coreV1 := r.Clientset.CoreV1()

	pod, err := coreV1.Pods(r.Namespace).Get(ctx, podName, metav1.GetOptions{})
	return pod, err
}

func (r *Descriptor) UpdateConfigMap(cfgMapName string, data map[string]string) (ret *v1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()

	cfgMap, err := coreV1.ConfigMaps(r.Namespace).Get(ctx, cfgMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		cfgMap.Data[key] = value
	}
	ret, err = coreV1.ConfigMaps(r.Namespace).Update(ctx, cfgMap, metav1.UpdateOptions{})
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
	ret, err = coreV1.ConfigMaps(r.Namespace).Create(ctx, &cfgMap, metav1.CreateOptions{})
	return ret, err
}

func (r *Descriptor) GetConfigMap(cfgMapName string) (ret *v1.ConfigMap, err error) {
	coreV1 := r.Clientset.CoreV1()
	ret, err = coreV1.ConfigMaps(r.Namespace).Get(ctx, cfgMapName, metav1.GetOptions{})
	return ret, err
}

func (r *Descriptor) AppendToExistingConfigMapsInPod(podName string, data map[string]string, overwrite bool) (ret []*v1.ConfigMap, err error) {
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
				if !overwrite {
					for key := range data {
						_, ok := cfgMap.Data[key]
						if ok {
							delete(tmpData, key)
						}
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

func (r *Descriptor) DeletePod(podName string, options metav1.DeleteOptions) error {
	coreV1 := r.Clientset.CoreV1()
	err := coreV1.Pods(r.Namespace).Delete(ctx, podName, options)
	return err
}

func New(namespace string, fieldSelector string, configPath string, clientset *kubernetes.Clientset) (r *Descriptor, err error) {
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
