package resources

import (
	"errors"
	"log"
	"testing"
	"time"

	"k8s.io/client-go/informers"
)

func fatal(t *testing.T, expected, got interface{}) {
	t.Helper()
	t.Fatalf(`expected: %v, got: %v`, expected, got)
}

func Test_Resources(t *testing.T) {
	tt := []struct {
		namespace        string
		name             string
		data             map[string]string
		expectedErr      error
		expectedResponse string
	}{
		{
			namespace:        "profiler",
			name:             "profiler-client-daemonset-9wvn2",
			data:             map[string]string{"CUDA_VISIBLE_DEVICES": "test"},
			expectedErr:      nil,
			expectedResponse: "",
		},
	}
	for i := range tt {
		tc := tt[i]

		t.Run(tc.name, func(t *testing.T) {
			desc, err := New("", "", "/home/dimitris/.kube/config", nil)
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
			_, err = desc.CreateConfigMap("woohoo", map[string]string{"1": "2"})
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
		})
	}
}

func Test_AppendToExistingConfigMapsInPod(t *testing.T) {
	tt := []struct {
		name             string
		podName          string
		namespace        string
		data             map[string]string
		overwrite        bool
		expectedErr      error
		expectedResponse string
	}{
		{
			name:        "test1",
			podName:     "mlperf-gpu-onnx-mobilenet-1024",
			namespace:   "default",
			data:        map[string]string{"CUDA_VISIBLE_DEVICES": "test"},
			overwrite:   true,
			expectedErr: nil,
		},
	}
	for i := range tt {
		tc := tt[i]

		t.Run(tc.name, func(t *testing.T) {
			desc, err := New(tc.namespace, "", "/home/dimitris/.kube/config", nil)
			if err != nil {
				log.Fatal(err)
			}
			factory := informers.NewSharedInformerFactory(desc.Clientset, 10*time.Minute)
			podIndexer := factory.Core().V1().Pods().Informer().GetIndexer()
			configMapIndexer := factory.Core().V1().ConfigMaps().Informer().GetIndexer()
			stopCh := make(chan struct{})
			factory.Start(stopCh)
			factory.WaitForCacheSync(stopCh)

			err = desc.AppendToExistingConfigMapsInPod(podIndexer, configMapIndexer, tc.podName, tc.data, tc.overwrite)
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
		})
	}
}

func Test_UpdateConfigMap(t *testing.T) {
	tt := []struct {
		name        string
		namespace   string
		cfgMapName  string
		data        map[string]string
		overwrite   bool
		expectedErr error
	}{
		{
			name:        "test2",
			namespace:   "default",
			cfgMapName:  "game-demo1",
			data:        map[string]string{"CUDA_VISIBLE_DEVICES": "test"},
			overwrite:   true,
			expectedErr: nil,
		},
	}
	for i := range tt {
		tc := tt[i]

		t.Run(tc.name, func(t *testing.T) {
			desc, err := New(tc.namespace, "", "/home/dimitris/.kube/config", nil)
			if err != nil {
				log.Fatal(err)
			}

			factory := informers.NewSharedInformerFactory(desc.Clientset, 10*time.Minute)
			indexer := factory.Core().V1().ConfigMaps().Informer().GetIndexer()
			stopCh := make(chan struct{})
			factory.Start(stopCh)
			factory.WaitForCacheSync(stopCh)
			_, err = desc.UpdateConfigMap(indexer, tc.cfgMapName, tc.data, tc.overwrite)
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
		})
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
