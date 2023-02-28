package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dimgatz98/k8s-gpu-scheduler/utils"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/redis/client"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

var desc *client.Descriptor

func main() {
	var nodeName, podName, namespace, uuids string
	fmt.Scanln(&nodeName)
	fmt.Scanln(&podName)
	fmt.Scanln(&namespace)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		uuids = scanner.Text()
	}
	uuids = strings.ReplaceAll(uuids, "'", "")
	uuids = strings.ReplaceAll(uuids, " ", "")
	uuids = strings.ReplaceAll(uuids, "[", "")
	uuids = strings.ReplaceAll(uuids, "]", "")
	uuidSlice := strings.Split(uuids, ",")
	// If GPU is MIG enabled maintain only MIG uuids
	if utils.Exists(uuidSlice, "MIG") != -1 {
		newUuids := []string{}
		for _, uuid := range uuidSlice {
			if strings.Contains(uuid, "MIG") {
				newUuids = append(newUuids, uuid)
			}
		}
		uuidSlice = newUuids
	}
	klog.Info("UUIDs: ", uuidSlice)

	var clientset *kubernetes.Clientset = nil
	clientset, err := utils.CheckClientset(clientset)
	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	nodeIndexer := factory.Core().V1().Nodes().Informer().GetIndexer()
	podsLister := factory.Core().V1().Pods().Lister()
	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)

	// Set node's UUIDs in redis
	redisUrls, err := utils.FindNodesIPFromPod(nodeIndexer, podsLister, "-0", "redis", "", nil, nil)
	if err != nil {
		log.Fatal("Error in FindNodesIP() in profiler client: ", err.Error())
	}
	var redisUrl string
	for _, value := range redisUrls[0] {
		redisUrl = value
	}

	// Add redis port and password in a k8s secret
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
