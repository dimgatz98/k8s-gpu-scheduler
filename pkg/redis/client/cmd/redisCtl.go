package main

import (
	"context"
	"fmt"
	"k8s-gpu-scheduler/pkg/pods"
	"k8s-gpu-scheduler/pkg/redis/client"
	"log"
	"os"
	"strings"

	"github.com/akamensky/argparse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func FindNodesIP(podNameContains string, namespace string, fieldSelector string) (string, error) {
	desc, err := pods.New(namespace, fieldSelector, "/home/dimitris/.kube/config")
	if err != nil {
		return "", err
	}
	podsList, err := desc.ListPods()
	if err != nil {
		return "", err
	}

	var url string
	for _, pod := range podsList.Items {
		if strings.Contains(pod.GetName(), podNameContains) {
			promNode, err := desc.Clientset.CoreV1().Nodes().Get(
				context.TODO(),
				pod.Spec.NodeName,
				metav1.GetOptions{},
			)
			if err != nil {
				return "", err
			}
			url = promNode.Status.Addresses[0].Address
			break
		}
	}
	return url, nil
}

func main() {
	parser := argparse.NewParser("Profiler", "Send profile gpu profiling data to gRPC server")

	flush := parser.Flag("f", "flush", &argparse.Options{Default: false, Help: "Flush redis database"})
	list := parser.Flag("l", "list", &argparse.Options{Default: false, Help: "List redis' data"})

	// Parse arguments
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Printf(parser.Usage(err))
		panic(err)
	}

	redisUrl, err := FindNodesIP("-0", "redis", "")
	if err != nil {
		klog.Info("FindNodesIP() failed in PostBind: ", err.Error())
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
