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

configurations_recommender = None
configurations_train_set = None

configurations_new_recommender = None
configurations_new_train_set = None

configurations_prev_version = None

terminate = False

configurations_path = os.getenv(
    "CONFIGURATIONS_DATA_PATH", default="recommender/configurations_train.ods"
)


interference_recommender = None
interference_train_set = None
interference_new_recommender = None
interference_new_train_set = None
interference_prev_version = None

terminate = False

interference_path = os.getenv(
    "INTERFERENCE_DATA_PATH", default="recommender/interference_train.ods"
)

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


def get_version(configurations_):
    with open(configurations_, "rb") as f:
        return hashlib.md5(f.read()).hexdigest()


def find_index_for_request(request, index):
    for ind in index:
        if ind in request:
            return ind
    return ""


def job():
    # Retrain every 5 mins
    def retrain(scheduler):
        global terminate, interference_new_recommender, configurations_new_recommender, interference_new_train_set, configurations_new_train_set, interference_prev_version, configurations_prev_version, interference_path, configurations_path
        if terminate:
            return

        scheduler.enter(JOB_DELAY, 1, retrain, (scheduler,))

        # Check if the version of the file has changed
        curr_version = get_version(configurations_path)
        if not os.path.exists(configurations_path) or not os.path.isfile(
            configurations_path
        ):
            print("Train data not found")
            print("Train data path: ", configurations_path)
        elif (
            configurations_prev_version is not None
            and configurations_prev_version == curr_version
        ):
            print("Train data haven't changed, not training")

        else:
            configurations_prev_version = curr_version
            configurations_new_train_set = pd.read_csv(configurations_path, sep="\t")
            configurations_new_train_set = configurations_new_train_set.set_index(
                "index"
            )
            configurations_train_set_arr = configurations_new_train_set.to_numpy()

            configurations_new_recommender = Imputer()
            configurations_new_recommender.fit(configurations_train_set_arr)

            print("Retrained configurations...")

        # Check if the version of the file has changed
        curr_version = get_version(interference_path)
        if not os.path.exists(interference_path) or not os.path.isfile(
            interference_path
        ):
            print("Train data not found")

        elif (
            interference_prev_version is not None
            and interference_prev_version == curr_version
        ):
            print("Train data haven't changed, not training")
        else:
            interference_prev_version = curr_version
            interference_new_train_set = pd.read_csv(interference_path, sep="\t")
            interference_new_train_set = interference_new_train_set.set_index("index")
            interference_train_set_arr = interference_new_train_set.to_numpy()

            interference_new_recommender = Imputer()
            interference_new_recommender.fit(interference_train_set_arr)

            print("Retrained Interference...")

    scheduler = sched.scheduler(time.time, time.sleep)
    scheduler.enter(JOB_DELAY, 1, retrain, (scheduler,))
    scheduler.run()


class recommenderServicer(recom_pb2_grpc.recommenderServicer):
    def ImputeConfigurations(self, request, context):
        global configurations_recommender, configurations_train_set, configurations_new_recommender, configurations_new_train_set

        if (
            configurations_new_recommender is not None
            and configurations_new_train_set is not None
        ):
            configurations_recommender = configurations_new_recommender
            configurations_train_set = configurations_new_train_set
            configurations_new_train_set = None
            configurations_new_recommender = None

        if configurations_recommender is None or configurations_train_set is None:
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        index = request.index.replace("-", "_")
        index = find_index_for_request(index, configurations_train_set.index)
        if index == "":
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        test_set_arr = (configurations_train_set.loc[index].to_numpy()).reshape(1, -1)

        res = configurations_recommender.predict(test_set_arr)[0]
        print("Imputed matrix:\n", res)

        reply = recom_pb2.Reply()
        set_protobuf_reply(res, configurations_train_set.columns, reply)

        return reply

    def ImputeInterference(self, request, context):
        global interference_recommender, interference_train_set, interference_new_recommender, interference_new_train_set

        if (
            interference_new_recommender is not None
            and interference_new_train_set is not None
        ):
            interference_recommender = interference_new_recommender
            interference_train_set = interference_new_train_set
            interference_new_train_set = None
            interference_new_recommender = None

        if interference_recommender is None or interference_train_set is None:
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        index = request.index.replace("-", "_")
        index = find_index_for_request(index, interference_train_set.index)
        if index == "":
            reply = recom_pb2.Reply()
            reply.result.append(0)
            return reply

        test_set_arr = (interference_train_set.loc[index].to_numpy()).reshape(1, -1)

        res = interference_recommender.predict(test_set_arr)[0]
        print("Imputed matrix:\n", res)

        reply = recom_pb2.Reply()
        set_protobuf_reply(res, interference_train_set.columns, reply)

        return reply


def serve():
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
    recom_pb2_grpc.add_recommenderServicer_to_server(recommenderServicer(), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    # Train for the first time
    if os.path.exists(configurations_path) or os.path.isfile(configurations_path):
        configurations_train_set = pd.read_csv(configurations_path, sep="\t")
        configurations_train_set = configurations_train_set.set_index("index")
        configurations_train_set_arr = configurations_train_set.to_numpy()
        configurations_prev_version = get_version(configurations_path)

        configurations_recommender = Imputer()
        configurations_recommender.fit(configurations_train_set_arr)
        print("Trained Configurations...")

    if os.path.exists(interference_path) or os.path.isfile(interference_path):
        interference_train_set = pd.read_csv(interference_path, sep="\t")
        interference_train_set = interference_train_set.set_index("index")
        interference_train_set_arr = interference_train_set.to_numpy()
        interference_prev_version = get_version(interference_path)

        interference_recommender = Imputer()
        interference_recommender.fit(interference_train_set_arr)
        print("Trained Interference...")

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
