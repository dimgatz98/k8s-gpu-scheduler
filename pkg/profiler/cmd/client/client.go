package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"k8s-gpu-scheduler/pkg/redis/client"
	"k8s-gpu-scheduler/utils"
	"log"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

var desc *client.Descriptor

func main() {
	var nodeName, podName, namespace, memFree, memTotal, smCount, uuids string
	fmt.Scanln(&nodeName)
	fmt.Scanln(&podName)
	fmt.Scanln(&namespace)
	fmt.Scanln(&memFree)
	fmt.Scanln(&memTotal)
	fmt.Scanln(&smCount)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		uuids = scanner.Text()
	}
	uuids = strings.ReplaceAll(uuids, "'", "")
	uuids = strings.ReplaceAll(uuids, " ", "")
	uuids = strings.ReplaceAll(uuids, "[", "")
	uuids = strings.ReplaceAll(uuids, "]", "")
	uuidSlice := strings.Split(uuids, ",")
	klog.Info("UUIDs: ", uuidSlice)

	// Set node's UUIDs in redis
	redisUrl, err := utils.FindNodesIP("-0", "redis", "")
	if err != nil {
		log.Fatal("Error in FindNodesIP() in profiler client: ", err.Error())
	}
	// Add redis password in a secret or environment variable and load from there
	desc = client.New(redisUrl+":32767", "1234", 0)
	byteArray, err := json.Marshal(uuidSlice)
	if err != nil {
		log.Fatal("Error in json.Marshal() in profiler client: ", err.Error())
	}
	jsonString := string(byteArray)

	err = desc.Set(nodeName, jsonString)
	if err != nil {
		log.Fatal("Error in redis Set() in profiler client: ", err.Error())
	}
}
