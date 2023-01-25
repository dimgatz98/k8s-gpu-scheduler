#!/bin/bash

parent_path=$( cd "$(dirname "${BASH_SOURCE[0]}")" ; pwd -P )
cd $parent_path

# Apply redis
kubectl apply -f ./deploy/redis
# Create and push profiler
cd pkg/profiler
make all
cd $root
# Apply profiler
kubectl apply -f ./deploy/profiler
# Create and push scheduler
make all
# Apply scheduler
kubectl apply -f deploy
