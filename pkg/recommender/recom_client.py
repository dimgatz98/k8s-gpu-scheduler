#!/usr/bin/python3

import recom_pb2_grpc
import recom_pb2

import grpc
import pandas as pd


def run():
    with grpc.insecure_channel(
        "localhost:50051", options=(("grpc.enable_http_proxy", 0),)
    ) as channel:
        data = pd.read_csv("./recommender/qps.ods", sep="\t")
        data = data.set_index("index")
        stub = recom_pb2_grpc.RecommenderStub(channel)

        # Add index in request
        request = recom_pb2.Request()
        request.index = data.index[0]

        reply = stub.Impute(request)
        print("Imputer Response:")
        print(reply)


if __name__ == "__main__":
    run()
