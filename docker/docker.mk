#
# Copyright 2022 The KubeBlocks Authors
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#     http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

# To use buildx: https://github.com/docker/buildx#docker-ce
export DOCKER_CLI_EXPERIMENTAL=enabled

DEBIAN_MIRROR=mirrors.aliyun.com


# Docker image build and push setting
DOCKER:=docker
DOCKERFILE_DIR?=./docker

LB_IMG ?= docker.io/apecloud/loadbalancer
LB_VERSION ?= 0.1.0
LB_TAG ?= v$(LB_VERSION)

# Image URL to use all building/pushing image targets
IMG ?= docker.io/apecloud/$(APP_NAME)
CLI_IMG ?= docker.io/apecloud/dbctl
CLI_TAG ?= v$(CLI_VERSION)

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG ?= latest
DEV_CONTAINER_IMAGE_NAME = docker.io/apecloud/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE = Dockerfile-dapr-dev
DOCKERFILE_DIR = ./docker

.PHONY: build-dev-image
build-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
build-dev-image: ## Build dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
endif


.PHONY: push-dev-image
push-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
push-dev-image: ## Push dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push
endif


.PHONY: build-cli-image
build-cli-image: clean-dbctl build-checks bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64 ## Build dbctl CLI container image.
	docker build . -t ${CLI_IMG}:${CLI_TAG} -f $(DOCKERFILE_DIR)/Dockerfile-dbctl

.PHONY: push-cli-image
push-cli-image: clean-dbctl build-checks bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64 ## Push dbctl CLI container image.
	docker push ${CLI_IMG}:${CLI_TAG}


.PHONY: build-manager-image
build-manager-image: test ## Build Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile -t ${IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION}
endif
endif


.PHONY: push-manager-image
push-manager-image: ## Push Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${IMG}:latest
else
	docker push ${IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest --push
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} --push
endif
endif

.PHONY: build-loadbalancer-image
build-loadbalancer-image: ## Push docker image with the loadbalancer.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${LB_IMG}:${LB_TAG} -t ${LB_IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${LB_IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${LB_IMG}:${LB_TAG} -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer
endif
endif

.PHONY: push-loadbalancer-image
push-loadbalancer-image: test ## Push docker image with the loadbalancer.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${LB_IMG}:latest
else
	docker push ${LB_IMG}:${LB_TAG}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${LB_IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer --push
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${LB_IMG}:${LB_TAG} -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer --push
endif
endif

