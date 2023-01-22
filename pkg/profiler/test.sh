#!/bin/bash

OUT=$(nvidia-smi --format=csv --query-gpu=power.draw,utilization.gpu,temperature.gpu | bin/parse_smi_metrics.py)

NODE_NAME=1
POD_NAME=2
POD_NAMESPACE=3
echo -e "$NODE_NAME\n$POD_NAME\n$POD_NAMESPACE\n$OUT\n$(bin/profiler)\n$(./parse_smi_uuids.py)" | go run pkg/client/client.go
