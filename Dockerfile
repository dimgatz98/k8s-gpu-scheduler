FROM debian:stretch-slim

WORKDIR /

COPY ./bin/gpu-sched /scheduler/bin/gpu-sched

CMD ["/scheduler/bin/gpu-sched"]
