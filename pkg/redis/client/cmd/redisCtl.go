package main

import (
	"fmt"
	"log"
	"os"

	"github.com/dimgatz98/k8s-gpu-scheduler/utils"

	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/redis/client"

	"github.com/akamensky/argparse"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {
	parser := argparse.NewParser("Redis control", "List and/or flush redis data")

	flush := parser.Flag("f", "flush", &argparse.Options{Default: false, Help: "Flush redis database"})
	list := parser.Flag("l", "list", &argparse.Options{Default: false, Help: "List redis' data"})
	configPath := parser.String("c", "config", &argparse.Options{Default: "/home/dimitris/.kube/config", Help: "Kubernetes config path"})

	// Parse arguments
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Printf(parser.Usage(err))
		panic(err)
	}

	var config *rest.Config
	config, err = clientcmd.BuildConfigFromFlags("", *configPath)
	if err != nil {
		log.Fatal("Error: ", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal("Error: ", err)
	}

	redisUrls, err := utils.FindNodesIPFromPod("-0", "redis", "", clientset)
	if err != nil {
		klog.Info("FindNodesIP() failed in PostBind: ", err.Error())
	}
	var redisUrl string
	for _, value := range redisUrls[0] {
		redisUrl = value
	}

	desc := client.New(redisUrl+":32767", "1234", 0)
	keys, err := desc.GetKeys()
	if err != nil {
		log.Fatal(err)
	}
	if *list {
		for _, key := range keys.([]interface{}) {
			data, err := desc.Get(key.(string))
			fmt.Println(key.(string) + ": " + data)
			if err != nil {
				klog.Info("Redis Get() failed in PostBind: ", err.Error())
			}
		}
	}

	if *flush {
		_ = desc.FlushRedis()
	}
}
