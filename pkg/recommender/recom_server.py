#!/usr/bin/python3

import sys

import signal
from concurrent import futures
import threading
from recommender.recommender import Imputer
import grpc
import recom_pb2
import recom_pb2_grpc
import pandas as pd
import sched, time
import os

from utils import set_protobuf_reply

import hashlib

recommender = None
train_set = None

new_recommender = None
new_train_set = None

prev_version = None

terminate = False

path = os.getenv("DATA_PATH")
try:
    port = int(os.getenv("PORT")) if os.getenv("PORT") is not None else 50051
except:
    port = 50051

JOB_DELAY = int(os.getenv("JOB_DELAY")) if os.getenv("JOB_DELAY") is not None else 30


def handler(signum, frame):
    global terminate
    print("\nSet terminate to false")
    terminate = True
    sys.exit(0)


def get_version(path):
    with open(path, "rb") as f:
        return hashlib.md5(f.read()).hexdigest()


def find_index_for_request(request, index):
    for ind in index:
        if ind in request:
            return ind
    return ""


def job():
    # Retrain every 5 mins
    def retrain(scheduler):
        global terminate, new_recommender, new_train_set, prev_version, path
        if terminate:
            return

        scheduler.enter(JOB_DELAY, 1, retrain, (scheduler,))

        if not os.path.exists(path) or not os.path.isfile(path):
            print("Train data not found")
            return

        # Check if the version of the file has changed
        curr_version = get_version(path)
        if prev_version is not None and prev_version == curr_version:
            print("Train data haven't changed, not training")
            return

        prev_version = curr_version
        new_train_set = pd.read_csv(path, sep="\t")
        new_train_set = new_train_set.set_index("index")
        train_set_arr = new_train_set.to_numpy()

        new_recommender = Imputer()
        new_recommender.fit(train_set_arr)

        print("Retrained...")

    scheduler = sched.scheduler(time.time, time.sleep)
    scheduler.enter(JOB_DELAY, 1, retrain, (scheduler,))
    scheduler.run()


class RecommenderServicer(recom_pb2_grpc.RecommenderServicer):
    def Impute(self, request, context):
        global recommender, train_set, new_recommender, new_train_set

        if new_recommender is not None and new_train_set is not None:
            recommender = new_recommender
            train_set = new_train_set
            new_train_set = None
            new_recommender = None

        if recommender is None or train_set is None:
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        index = request.index.replace("-", "_")
        index = find_index_for_request(index, train_set.index)
        if index == "":
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        test_set_arr = (train_set.loc[index].to_numpy()).reshape(1, -1)

        res = recommender.predict(test_set_arr)[0]
        print("Imputed matrix:\n", res)

        reply = recom_pb2.Reply()
        set_protobuf_reply(res, train_set.columns, reply)

        return reply


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    recom_pb2_grpc.add_RecommenderServicer_to_server(RecommenderServicer(), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    # Train for the first time
    if os.path.exists(path) or os.path.isfile(path):
        train_set = pd.read_csv(path, sep="\t")
        train_set = train_set.set_index("index")
        train_set_arr = train_set.to_numpy()
        prev_version = get_version(path)

        recommender = Imputer()
        recommender.fit(train_set_arr)
        print("Trained...")

    # Start thread to scheduler retrains
    x = threading.Thread(
        target=job,
    )
    x.start()
    # Signal used to set terminate to true and terminate thread
    signal.signal(signal.SIGINT, handler)

    ## Add thread to watch pods add rows or update missing fields in train.ods

    # Start gRPC server
    serve()
