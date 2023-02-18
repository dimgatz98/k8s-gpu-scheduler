REPO=tzourhs
BIN_DIR=bin
TAG=1.54
IMAGE=gpu-scheduler

.EXPORT_ALL_VARIABLES:

init:
	mkdir -p ${BIN_DIR}

local: update  init
	go build -o=${BIN_DIR}/gpu-sched ./cmd/scheduler/main.go

build-linux: init
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o=${BIN_DIR}/gpu-sched ./cmd/scheduler/main.go

all: update build-linux image-build image-push	

image-build:
	docker build --no-cache . -t $(REPO)/$(IMAGE):$(TAG)

image-push:
	docker push $(REPO)/$(IMAGE):$(TAG)

update:
	# go mod download
	go mod tidy
	go mod vendor

clean:
	rm -rf _output/
	rm -f *.log
