TAG=1.11
REPO=tzourhs

create-bin:
	mkdir -p bin

build-client:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/client cmd/client/client.go

copy-files:
	cp parse_smi_uuids.py parse_smi_metrics.py profile_gpu.sh bin/

compile-cuda:
	nvcc gpu_profiling.cpp -o bin/profiler

build: create-bin compile-cuda copy-files build-client

docker-client:
	docker build . -t $(REPO)/profiler_client:$(TAG)
	docker push $(REPO)/profiler_client:$(TAG)
	
update:
	# go mod download
	go mod tidy
	go mod vendor

all: update create-bin compile-cuda copy-files build-client docker-client

clean: 
	rm -rf bin
