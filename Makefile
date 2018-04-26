.PHONY: build release

IMAGE  := quay.io/outreach/limiter
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

build:
	docker build --tag $(IMAGE):$(BRANCH) .
	if [ "$(BRANCH)" = "master" ]; then
		docker tag $(IMAGE):$(BRANCH) $(IMAGE)
		docker tag $(IMAGE):$(BRANCH) $(IMAGE):latest
	fi

release: build
	docker push $(IMAGE):$(BRANCH)
	if [ "$(BRANCH)" = "master" ]; then
		docker push $(IMAGE)
		docker push $(IMAGE):latest
	fi
