package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"testing"
)

var desc Client

func fatal(t *testing.T, expected, got interface{}) {
	t.Helper()
	t.Fatalf(`expected: %v, got: %v`, expected, got)
}

func Test_Redis(t *testing.T) {
	tt := []struct {
		name                 string
		function             string
		data                 []byte
		expectedErr          error
		expectedResponse     string
		expectedResponseList []string
	}{
		{
			name:                 "test1",
			function:             "set",
			data:                 []byte(`{"key1": "value1"}`),
			expectedErr:          nil,
			expectedResponse:     "",
			expectedResponseList: nil,
		},
		{
			name:                 "test2",
			function:             "set",
			data:                 []byte(`{"key1": "value2"}`),
			expectedErr:          nil,
			expectedResponse:     "",
			expectedResponseList: nil,
		},
		{
			name:                 "test3",
			function:             "get",
			data:                 []byte("key1"),
			expectedErr:          nil,
			expectedResponse:     "value2",
			expectedResponseList: nil,
		},
		{
			name:                 "test4",
			function:             "getRange",
			data:                 []byte("key1"),
			expectedErr:          nil,
			expectedResponse:     "al",
			expectedResponseList: nil,
		},
		{
			name:                 "test5",
			function:             "extend",
			data:                 []byte("test"),
			expectedErr:          nil,
			expectedResponse:     "",
			expectedResponseList: nil,
		},
		{
			name:                 "test6",
			function:             "get",
			data:                 []byte("key1"),
			expectedErr:          nil,
			expectedResponse:     "value2test",
			expectedResponseList: nil,
		},
		{
			name:                 "test7",
			function:             "get",
			data:                 []byte("UUIDs"),
			expectedErr:          nil,
			expectedResponse:     "value2test",
			expectedResponseList: nil,
		},
		{
			name:                 "test8",
			function:             "getKeys",
			data:                 nil,
			expectedErr:          nil,
			expectedResponse:     "",
			expectedResponseList: []string{"key1", "key"},
		},
	}
	for i := range tt {
		tc := tt[i]

		t.Run(tc.name, func(t *testing.T) {
			if tc.function == "set" {
				var m map[string]string
				json.Unmarshal(tc.data, &m)
				log.Println("data: ", tc.data, "map: ", m)
				for key, value := range m {
					log.Println("key: ", key, "value: ", value)
					err := desc.Set(key, value)
					if !errors.Is(err, tc.expectedErr) {
						fatal(t, tc.expectedErr, err)
					}
					log.Println("Put ", value, " in redis for key: ", key)
				}
			} else if tc.function == "get" {
				val, err := desc.Get(string(tc.data))
				if !errors.Is(err, tc.expectedErr) {
					fatal(t, tc.expectedErr, err)
				}
				if !reflect.DeepEqual(val, tc.expectedResponse) {
					fatal(t, tc.expectedResponse, val)
				}
			} else if tc.function == "getRange" {
				val, err := desc.GetRange(string(tc.data), 1, 2)
				fmt.Println(val)
				if !errors.Is(err, tc.expectedErr) {
					fatal(t, tc.expectedErr, err)
				}
				if !reflect.DeepEqual(val, tc.expectedResponse) {
					fatal(t, tc.expectedResponse, val)
				}
			} else if tc.function == "extend" {
				val, err := desc.Get("key1")
				if !errors.Is(err, tc.expectedErr) {
					fatal(t, tc.expectedErr, err)
				}
				data := fmt.Sprintf("%s%s", val, string(tc.data))
				m := map[string]string{"key1": data}
				log.Println("data: ", data, "map: ", m)
				for key, value := range m {
					log.Println("key: ", key, "value: ", value)
					err := desc.Set(key, value)
					if !errors.Is(err, tc.expectedErr) {
						fatal(t, tc.expectedErr, err)
					}
					log.Println("Put ", value, " in redis for key: ", key)
				}
			} else {
				keys, err := desc.GetKeys()
				fmt.Println(keys)
				if !errors.Is(err, tc.expectedErr) {
					fatal(t, tc.expectedErr, err)
				}
			}
		})
	}
}

func TestMain(m *testing.M) {
	desc = New("172.20.0.5:32767", "1234", 0)
	m.Run()
}
