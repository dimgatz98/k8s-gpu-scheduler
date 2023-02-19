package client

import (
	"context"

	recommender "github.com/dimgatz98/k8s-gpu-scheduler/pkg/recommender/go_client"

	"google.golang.org/grpc"
)

func Call(url string, index string) (reply *recommender.Reply, err error) {
	var conn *grpc.ClientConn
	conn, err = grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	c := recommender.NewRecommenderClient(conn)

	response, err := c.ImputeConfigurations(context.Background(), &recommender.Request{Index: index})
	return response, err
}
