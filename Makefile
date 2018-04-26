.PHONY: build release

IMAGE := quay.io/outreach/limiter

build:
	docker build -t $(IMAGE) .

release: build
	docker push $(IMAGE)
