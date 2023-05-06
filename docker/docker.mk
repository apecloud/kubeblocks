#
#Copyright (C) 2022-2023 ApeCloud Co., Ltd
#
#This file is part of KubeBlocks project
#
#This program is free software: you can redistribute it and/or modify
#it under the terms of the GNU Affero General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.
#
#This program is distributed in the hope that it will be useful
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU Affero General Public License for more details.
#
#You should have received a copy of the GNU Affero General Public License
#along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

# To use buildx: https://github.com/docker/buildx#docker-ce
export DOCKER_CLI_EXPERIMENTAL=enabled

DEBIAN_MIRROR=mirrors.aliyun.com


# Docker image build and push setting
DOCKER:=docker
DOCKERFILE_DIR?=./docker

# Image URL to use all building/pushing image targets
IMG ?= docker.io/apecloud/$(APP_NAME)
TOOL_IMG ?= docker.io/apecloud/$(APP_NAME)-tool
CLI_IMG ?= docker.io/apecloud/kbcli
CLI_TAG ?= v$(CLI_VERSION)

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG ?= latest
DEV_CONTAINER_IMAGE_NAME = docker.io/apecloud/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE = Dockerfile-dev
DOCKERFILE_DIR = ./docker
BUILDX_ARGS ?=

##@ Docker containers

.PHONY: build-dev-container-image
build-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
build-dev-image: ## Build dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) -f $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build $(DOCKERFILE_DIR)/.  $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) $(BUILDX_ARGS)
endif


.PHONY: push-dev-container-image
push-dev-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
push-dev-image: ## Push dev container image.
ifneq ($(BUILDX_ENABLED), true)
	docker push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build . $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push $(BUILDX_ARGS)
endif


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

.PHONY: build-tools-image
build-tools-image: generate ## Build tools container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build . -t ${TOOL_IMG}:${VERSION} -f $(DOCKERFILE_DIR)/Dockerfile-tools -t ${TOOL_IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${TOOL_IMG}:latest $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${TOOL_IMG}:${VERSION} $(BUILDX_ARGS)
endif
endif

.PHONY: push-tools-image
push-tools-image: generate ## Push tools container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	docker push ${TOOL_IMG}:latest
else
	docker push ${TOOL_IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${TOOL_IMG}:latest --push $(BUILDX_ARGS)
else
	docker buildx build . -f $(DOCKERFILE_DIR)/Dockerfile-tools $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -t ${TOOL_IMG}:${VERSION} --push $(BUILDX_ARGS)
endif
endif
