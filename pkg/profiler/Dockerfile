FROM nvidia/cuda:12.0.0-base-ubuntu20.04

COPY bin /client

RUN apt update && apt install -y python3

CMD /client/profile_gpu.sh
