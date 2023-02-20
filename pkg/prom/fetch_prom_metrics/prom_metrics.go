package metrics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/prom/requests"

	"k8s.io/klog/v2"
)

type Response struct {
	MetricName string
	Exporter   string
	Value      string
	GPU_I_ID   string
	UUID       string
}

func ParseResponse(response []byte) (Responses []Response, err error) {
	var resp interface{}
	if len(response) == 0 {
		return nil, nil
	}

	err = json.Unmarshal(response, &resp)
	if err != nil {
		return nil, err
	}
	results := resp.(map[string]interface{})["data"].(map[string]interface{})["result"].([]interface{})
	if len(results) == 0 {
		return nil, nil
	}
	var metric_name, exporter, value, gpuID, uuid string
	var metric map[string]interface{}
	for _, result := range results {
		value = result.(map[string]interface{})["value"].([]interface{})[1].(string)
		metric = result.(map[string]interface{})["metric"].(map[string]interface{})
		metric_name = metric["__name__"].(string)

		val, ok := metric["GPU_I_ID"]
		if ok {
			gpuID = val.(string)
		} else {
			gpuID = ""
		}

		val, ok = metric["UUID"]
		if ok {
			uuid = val.(string)
		} else {
			uuid = ""
		}

		exporter = metric["pod"].(string)
		Responses = append(Responses, Response{MetricName: metric_name, Value: value, Exporter: exporter, GPU_I_ID: gpuID, UUID: uuid})
	}
	return Responses, nil
}

func DcgmPromInstantQuery(url string, filter string) ([]Response, error) {
	params := []map[string]string{
		{"query": "DCGM_FI_PROF_GR_ENGINE_ACTIVE"},
		{"query": "DCGM_FI_DEV_MEM_COPY_UTIL"},
		{"query": "DCGM_FI_DEV_GPU_TEMP"},
		{"query": "DCGM_FI_DEV_FB_USED"},
		{"query": "DCGM_FI_DEV_FB_FREE"},
	}

	if filter != "" {
		for i := range params {
			for key := range params[i] {
				params[i][key] += filter
			}
		}
	}

	path := []string{}
	for i := 0; i < len(params); i++ {
		path = append(path, "api/v1/query")
	}

	var responses [][]byte
	req := requests.New(url, http.DefaultClient, time.Second)
	respChannel := make(chan []byte)
	errChannel := make(chan error)
	for i := 0; i < len(params); i++ {
		go func(cnt int) {
			resp, err := req.Request(context.Background(), path[cnt], params[cnt])
			if err != nil {
				errChannel <- err
			}
			if resp != nil {
				respChannel <- resp
			}
		}(i)
	}

	for i := 0; i < len(params); i++ {
		select {
		case resp := <-respChannel:
			responses = append(responses, resp)
		case err := <-errChannel:
			klog.Info("Prometheus api returned error: ", err)
		}
	}

	var Responses []Response
	var tmp []Response
	var err error
	for _, response := range responses {
		tmp, err = ParseResponse(response)
		Responses = append(Responses, tmp...)
	}
	return Responses, err
}
