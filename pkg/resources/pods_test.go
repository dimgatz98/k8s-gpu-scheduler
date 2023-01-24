package resources

import (
	"errors"
	"testing"
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
			_, err = desc.CreateConfigMap("wooho", map[string]string{"1": "2"})
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
		})
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
