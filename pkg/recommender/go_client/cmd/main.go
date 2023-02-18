package main

import (
	"log"

	client "github.com/dimgatz98/k8s-gpu-scheduler/pkg/recommender/go_client/pkg"
	"github.com/dimgatz98/k8s-gpu-scheduler/pkg/recommender/go_client/utils"
)

func main() {

	response, err := client.Call("172.20.0.3:32700", "onnx_mobilenet_1024")
	if err != nil {
		log.Fatalln("Error : ", err)
	}

	print(utils.FindMaxIndForNode("A30", response))
}
