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

# Docker image build and push setting
DOCKER:=docker
DOCKERFILE_DIR?=./docker


# To use buildx: https://github.com/docker/buildx#docker-ce
export DOCKER_CLI_EXPERIMENTAL=enabled

# check the required environment variables
check-docker-env:
ifeq ($(DAPR_REGISTRY),)
	$(error DAPR_REGISTRY environment variable must be set)
endif
ifeq ($(DAPR_TAG),)
	$(error DAPR_TAG environment variable must be set)
endif

check-arch:
ifeq ($(TARGET_OS),)
	$(error TARGET_OS environment variable must be set)
endif
ifeq ($(TARGET_ARCH),)
	$(error TARGET_ARCH environment variable must be set)
endif


################################################################################
# Target: build-dev-container, push-dev-container                              #
################################################################################

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG?=latest

# Use this to pin a specific version of the Dapr CLI to a devcontainer
DEV_CONTAINER_CLI_TAG?=1.8.0

# Dapr container image name
DEV_CONTAINER_IMAGE_NAME= docker.io/infracreate/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE=Dockerfile-dev
DOCKERFILE_DIR=./docker

check-docker-env-for-dev-container:
ifeq ($(DAPR_REGISTRY),)
	$(error DAPR_REGISTRY environment variable must be set)
endif

.PHONY: build-dev-container
build-dev-container: ## Build dev docker container image.
ifneq ($(BUILDX_ENABLED), true)
	docker build $(DOCKERFILE_DIR)/. -f $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
endif


.PHONY: push-dev-container
push-dev-container: ## Push dev docker container image.
ifneq ($(BUILDX_ENABLED), true)
	docker push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	docker buildx build $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) -f $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) -t $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push
endif
