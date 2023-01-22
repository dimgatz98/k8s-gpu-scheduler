package metrics

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func fatal(t *testing.T, expected, got interface{}) {
	t.Helper()
	t.Fatalf(`expected: %v, got: %v`, expected, got)
}

func Test_ParseResponse(t *testing.T) {
	tt := []struct {
		name             string
		filename         string
		expectedErr      error
		expectedResponse []Response
	}{
		{
			name:             "Test_ParseResponse1",
			filename:         "",
			expectedResponse: nil,
		},
		{
			name:        "Test_ParseResponse2",
			filename:    "../test_data/prom_response_mock.txt",
			expectedErr: nil,
			expectedResponse: []Response{
				{
					MetricName: "DCGM_FI_DEV_FB_FREE",
					Exporter:   "dcgm-exporter-1673788700-8xg5s",
					Value:      "4005",
				},
				{
					MetricName: "DCGM_FI_DEV_FB_FREE",
					Exporter:   "dcgm-exporter-1673788700-9g6m5",
					Value:      "4005",
				},
				{
					MetricName: "DCGM_FI_DEV_FB_FREE",
					Exporter:   "dcgm-exporter-1673788700-cf9nx",
					Value:      "4005",
				},
				{
					MetricName: "DCGM_FI_DEV_FB_FREE",
					Exporter:   "dcgm-exporter-1673788700-pcpm7",
					Value:      "4005",
				},
			},
		},
		{
			name:             "Test_ParseResponse3",
			filename:         "../test_data/empty_response.txt",
			expectedErr:      nil,
			expectedResponse: nil,
		},
	}
	for i := range tt {
		tc := tt[i]
		data, err := os.ReadFile(tc.filename)
		if err != nil {
			data = nil
		}

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseResponse(data)
			if !errors.Is(err, tc.expectedErr) {
				fatal(t, tc.expectedErr, err)
			}
			if !reflect.DeepEqual(got, tc.expectedResponse) {
				fatal(t, tc.expectedResponse, got)
			}
		})
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
