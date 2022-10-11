#
# Copyright 2022 The Kubeblocks Authors
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
IMG ?= docker.io/infracreate/$(APP_NAME)
CLI_IMG ?= docker.io/infracreate/dbctl
CLI_TAG ?= v$(CLI_VERSION)

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG ?= latest
DEV_CONTAINER_IMAGE_NAME = docker.io/infracreate/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE = Dockerfile-dapr-dev
DOCKERFILE_DIR = ./docker

.PHONY: build-dev-container
build-dev-container: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
build-dev-container: ## Build dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
endif


.PHONY: push-dev-container
push-dev-container: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
push-dev-container: ## Push dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push
endif


.PHONY: build-cli-container
build-cli-container: clean-dbctl build-checks bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64 ## Build dbctl CLI container image.
	docker build . -t ${CLI_IMG}:${CLI_TAG} -f $(DOCKERFILE_DIR)/Dockerfile-dbctl

.PHONY: push-cli-container
push-cli-container: clean-dbctl build-checks bin/dbctl.linux.amd64 bin/dbctl.linux.arm64 bin/dbctl.darwin.arm64 bin/dbctl.darwin.amd64 bin/dbctl.windows.amd64 ## Push dbctl CLI container image.
	docker push ${CLI_IMG}:${CLI_TAG}


.PHONY: build-manager-container
build-manager-container: test ## Build Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile -t ${IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:latest
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${IMG}:${VERSION}
endif
endif


.PHONY: push-manager-container
push-manager-container: ## Push Operator manager container image.
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