package requests

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	server *httptest.Server
	req    Requests
)

func fatal(t *testing.T, expected, got interface{}) {
	t.Helper()
	t.Fatalf(`expected: %v, got: %v`, expected, got)
}

func mockPromInstantQuery(w http.ResponseWriter, r *http.Request) {
	status := http.StatusOK

	params, ok := r.URL.Query()["query"]
	response := make(map[string]interface{})
	if !ok || len(params[0]) == 0 {
		status = http.StatusBadRequest
	} else {
		response["test"] = "mock"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}

func Test_Request(t *testing.T) {
	tt := []struct {
		name             string
		path             string
		params           map[string]string
		expectedResponse []byte
		expectedErr      error
	}{
		{
			name:             "test1",
			path:             "api/v1/query",
			params:           map[string]string{"query": "DCGM_FI_DEV_GPU_UTIL"},
			expectedResponse: []byte(`{"test":"mock"}` + "\n"),
			expectedErr:      nil,
		},
	}

	for i := range tt {
		tc := tt[i]
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotResponse, gotErr := req.Request(context.Background(), tc.path, tc.params)

			if !errors.Is(gotErr, tc.expectedErr) {
				fatal(t, tc.expectedErr, gotErr)
			}

			if !reflect.DeepEqual(gotResponse, tc.expectedResponse) {
				fatal(t, tc.expectedResponse, gotResponse)
			}
		})
	}
}

func TestMain(m *testing.M) {
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimSpace(r.URL.Path) {
		case "/api/v1/query":
			mockPromInstantQuery(w, r)
		default:
			http.NotFoundHandler().ServeHTTP(w, r)
		}
	}))
	req = New(server.URL, http.DefaultClient, time.Second)

	m.Run()
}
