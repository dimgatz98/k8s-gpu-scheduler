FROM ubuntu

COPY ./recom_pb2_grpc.py /recommender/recom_pb2_grpc.py
COPY ./recom_pb2.py /recommender/recom_pb2.py
COPY ./recom_server.py /recommender/recom_server.py
COPY ./requirements.txt /recommender/requirements.txt
COPY ./utils.py /recommender/utils.py
COPY ./recommender /recommender/recommender

ENV TZ=America/Los_Angeles
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN apt update\
    && apt -y upgrade
RUN apt install -y software-properties-common
RUN add-apt-repository ppa:deadsnakes/ppa\
    && apt-get update \
    && apt-get install -y python3.8 python3-pip python3.8-distutils python3-apt
RUN update-alternatives --install /usr/bin/python3 python3 \
    /usr/bin/python3.8 3
RUN python3 -m pip install --upgrade pip
RUN python3 -m pip install -r /recommender/requirements.txt --use-deprecated=legacy-resolver


CMD python3 /recommender/recom_server.py
