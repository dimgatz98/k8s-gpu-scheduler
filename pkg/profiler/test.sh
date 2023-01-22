#!/bin/bash

OUT="['GPU-ac0112df-7098-6c59-5c4f-a57fa666f808', 'MIG-7a700938-2114-5d88-a93c-167c2a498910', 'MIG-c8956631-ac65-59d3-9065-c8764799febf', 'MIG-266f4973-a8c6-5697-a549-dfc880e350c1', 'MIG-41e35365-768c-587c-a36e-65197e7a7f93']"
NODE_NAME=1
POD_NAME=2
POD_NAMESPACE=3
echo -e "$NODE_NAME\n$POD_NAME\n$POD_NAMESPACE\n$(bin/profiler)\n$OUT" | go run cmd/client/client.go
