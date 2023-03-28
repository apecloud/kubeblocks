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

# Image URL to use all building/pushing image targets
IMG ?= docker.io/apecloud/$(APP_NAME)
CLI_IMG ?= docker.io/apecloud/kbcli
CLI_TAG ?= v$(CLI_VERSION)
IMG_TOOLS ?= $(IMG)-tools

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG ?= latest
DEV_CONTAINER_IMAGE_NAME = docker.io/apecloud/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE = Dockerfile-dev
DOCKERFILE_DIR = ./docker
BUILDX_ARGS ?=

.PHONY: build-dev-image
build-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
build-dev-image: ## Build dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build $(DOCKERFILE_DIR)/.  $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) $(BUILDX_ARGS)
endif


.PHONY: push-dev-image
push-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
push-dev-image: ## Push dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push $(BUILDX_ARGS)
endif


.PHONY: build-cli-image
build-cli-image: clean-kbcli build-checks bin/kbcli.linux.amd64 bin/kbcli.linux.arm64 bin/kbcli.darwin.arm64 bin/kbcli.darwin.amd64 bin/kbcli.windows.amd64 ## Build kbcli container image.
	docker build . -t ${CLI_IMG}:${CLI_TAG} -f $(DOCKERFILE_DIR)/Dockerfile-cli

.PHONY: push-cli-image
push-cli-image: clean-kbcli build-checks bin/kbcli.linux.amd64 bin/kbcli.linux.arm64 bin/kbcli.darwin.arm64 bin/kbcli.darwin.amd64 bin/kbcli.windows.amd64 ## Push kbcli container image.
	docker push ${CLI_IMG}:${CLI_TAG}


.PHONY: build-manager-image
build-manager-image: generate ## Build Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile -t ${IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} $(BUILDX_ARGS)
endif
endif


.PHONY: push-manager-image
push-manager-image: generate ## Push Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${IMG}:latest
else
	docker push ${IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest --push $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} --push $(BUILDX_ARGS)
endif
endif

.PHONY: build-loadbalancer-image
build-loadbalancer-image: generate ## Push docker image with the loadbalancer.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG}:${VERSION} -t ${IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer $(BUILDX_ARGS)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer $(BUILDX_ARGS)
endif
endif

.PHONY: push-loadbalancer-image
push-loadbalancer-image: generate ## Push docker image with the loadbalancer.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${IMG}:latest
else
	docker push ${IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer --push $(BUILDX_ARGS)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile-loadbalancer --push $(BUILDX_ARGS)
endif
endif

.PHONY: build-manager-tools-image
build-manager-tools-image: generate ## Build Operator manager-tools container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG_TOOLS}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile-tools -t ${IMG_TOOLS}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG_TOOLS}:latest $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG_TOOLS}:${VERSION} $(BUILDX_ARGS)
endif
endif

.PHONY: push-manager-tools-image
push-manager-tools-image: generate ## Push Operator manager-tools container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${IMG_TOOLS}:latest
else
	docker push ${IMG_TOOLS}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG_TOOLS}:latest --push $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG_TOOLS}:${VERSION} --push $(BUILDX_ARGS)
endif
endif
