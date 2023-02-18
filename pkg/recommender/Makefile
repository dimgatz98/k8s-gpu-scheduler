REPO=tzourhs
TAG=0.7
IMAGE=recommender

all: install image-build image-push

install:
	pip install -r requirements.txt

docker: image-build image-push

image-build:
	docker build --no-cache . -t $(REPO)/$(IMAGE):$(TAG)

image-push:
	docker push $(REPO)/$(IMAGE):$(TAG)
