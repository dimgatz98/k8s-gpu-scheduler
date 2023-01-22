FROM debian:stretch-slim

WORKDIR /

COPY . /scheduler

CMD ["/scheduler/bin/gpu-sched"]
