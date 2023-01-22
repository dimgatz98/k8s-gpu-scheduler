package main

import (
	"encoding/json"
	"fmt"
	"gpu-scheduler/pkg/redis/client"
	"gpu-scheduler/utils"
	"log"
	"strings"
)

type GPUData struct {
	MemFree        string   `protobuf:"bytes,1,opt,name=memFree,proto3" json:"memFree,omitempty"`
	MemUsed        string   `protobuf:"bytes,2,opt,name=memUsed,proto3" json:"memUsed,omitempty"`
	SmCount        string   `protobuf:"bytes,3,opt,name=smCount,proto3" json:"smCount,omitempty"`
	GpuUtilization string   `protobuf:"bytes,4,opt,name=gpuUtilization,proto3" json:"gpuUtilization,omitempty"`
	Power          string   `protobuf:"bytes,5,opt,name=power,proto3" json:"power,omitempty"`
	Temp           string   `protobuf:"bytes,6,opt,name=temp,proto3" json:"temp,omitempty"`
	PodNode        string   `protobuf:"bytes,7,opt,name=podNode,proto3" json:"podNode,omitempty"`
	PodNamespace   string   `protobuf:"bytes,8,opt,name=podNamespace,proto3" json:"podNamespace,omitempty"`
	Pod            string   `protobuf:"bytes,9,opt,name=senderPod,proto3" json:"senderPod,omitempty"`
	Uuid           []string `protobuf:"bytes,10,rep,name=uuid,proto3" json:"uuid,omitempty"`
}

var desc *client.Descriptor

func main() {
	var nodeName, podName, namespace, memFree, memTotal, smCount, gpuUtil, uuids, temp, power string
	fmt.Scanln(&nodeName)
	fmt.Scanln(&podName)
	fmt.Scanln(&namespace)
	fmt.Scanln(&power)
	fmt.Scanln(&gpuUtil)
	fmt.Scanln(&temp)
	fmt.Scanln(&memFree)
	fmt.Scanln(&memTotal)
	fmt.Scanln(&smCount)
	fmt.Scanln(&uuids)
	uuids = strings.ReplaceAll(uuids, "'", "")
	uuids = strings.ReplaceAll(uuids, " ", "")
	uuids = strings.ReplaceAll(uuids, "[", "")
	uuids = strings.ReplaceAll(uuids, "]", "")
	uuidSlice := strings.Split(uuids, ",")

	data := GPUData{
		MemFree:        memFree,
		MemUsed:        memTotal,
		SmCount:        smCount,
		GpuUtilization: gpuUtil,
		Temp:           temp,
		Power:          power,
		Uuid:           uuidSlice,
		PodNode:        nodeName,
		Pod:            podName,
		PodNamespace:   namespace,
	}

	// Set node's UUIDs in redis
	redisUrl, err := utils.FindNodesIP("-0", "redis", "")
	if err != nil {
		log.Fatal("Error in FindNodesIP in profiler client: ", err.Error())
	}
	// Add redis password in a secret or environment variable and load from there
	desc = client.New(redisUrl+":32767", "1234", 0)
	byteArray, err := json.Marshal(data.Uuid)
	if err != nil {
		log.Fatal("Error in json.Marshal() in profiler client: ", err.Error())
	}
	jsonString := string(byteArray)

	err = desc.Set(data.PodNode, jsonString)
	if err != nil {
		log.Fatal("Error in redis Set() in profiler client: ", err.Error())
	}
}
