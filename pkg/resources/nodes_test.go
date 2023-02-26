package resources

import (
	"errors"
	"testing"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_Nodes(t *testing.T) {
	var config *rest.Config
	config, err := clientcmd.BuildConfigFromFlags("", "/home/dimitris/.kube/config")
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	tt := []struct {
		name             string
		nodeName         string
		operatorData     map[string]interface{}
		operatorPath     string
		operatorType     string
		expectedErr      error
		expectedResponse string
	}{
		{
			name:     "test_patch",
			nodeName: "k8s-aferik-gpu-a30",
			operatorData: map[string]interface{}{
				"nvidia.com/mig.config": "all-1g.6gb",
			},
			operatorPath:     "/metadata/labels",
			operatorType:     "replace",
			expectedErr:      nil,
			expectedResponse: "",
		},
	}
	for i := range tt {
		tc := tt[i]

		t.Run(tc.name, func(t *testing.T) {
			params := PatchNodeParam{
				Node:         tc.nodeName,
				OperatorType: tc.operatorType,
				OperatorPath: tc.operatorPath,
				OperatorData: tc.operatorData,
			}

			factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
			indexer := factory.Core().V1().Nodes().Informer().GetIndexer()
			stopCh := make(chan struct{})
			factory.Start(stopCh)
			factory.WaitForCacheSync(stopCh)

			_, err = params.LabelNode(indexer, clientset)
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
		})
	}
}
