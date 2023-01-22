#!/bin/bash

PREV=""
while true; do
    UUIDS=$(/client/parse_smi_uuids.py)
    if [ "PREV" = "UUIDS" ]; then
        sleep 2
        continue
    fi
    OUT=$(nvidia-smi --format=csv --query-gpu=power.draw,utilization.gpu,temperature.gpu | /client/parse_smi_metrics.py)
    echo -e "$NODE_NAME\n$POD_NAME\n$POD_NAMESPACE\n$OUT\n$(/client/profiler)\n$UUIDS" | /client/client -p 31111
    PREV=UUIDS
done