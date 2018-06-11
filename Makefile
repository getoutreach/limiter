.PHONY: build release

IMAGE  := registry.outreach.cloud/limiter
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

build: test
	docker build --tag $(IMAGE):$(BRANCH) .

test: lint
	go test -v

lint:
	golint

release: build
	docker push $(IMAGE):$(BRANCH)
