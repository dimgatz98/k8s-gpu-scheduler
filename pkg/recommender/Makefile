REPO=tzourhs
TAG=0.9
IMAGE=recommender

all: install image-build image-push

install:
	pip install -r requirements.txt --user

docker: image-build image-push

image-build:
	docker build --no-cache . -t $(REPO)/$(IMAGE):$(TAG)

image-push:
	docker push $(REPO)/$(IMAGE):$(TAG)
