IMAGES = api event-reporter eirini-controller task-reporter instance-index-env-injector migration resource-validator

TAG ?= latest
DOCKER_DIR := ${CURDIR}
EIRINI_DIR := ${DOCKER_DIR}/..
REVISION := $(shell git -C $(EIRINI_DIR) rev-parse HEAD)

.PHONY: $(IMAGES)

all: $(IMAGES)

$(IMAGES):
	DOCKER_BUILDKIT=1 docker build $(EIRINI_DIR) \
		--file "$(DOCKER_DIR)/$@/Dockerfile" \
		--build-arg GIT_SHA=$(REVISION) \
		--tag "eirini/$@:$(TAG)"

push:
	$(foreach img, $(IMAGES), docker push eirini/$(img):$(TAG);)

push-%:
	docker push eirini/$*:$(TAG)

clean:
	docker image prune --force
	docker rmi $(shell docker image ls --format="{{.Repository}}:{{.Tag}}" --filter=reference="eirini/*")

