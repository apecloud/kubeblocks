#
#Copyright (C) 2022-2024 ApeCloud Co., Ltd
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

# Debian APT mirror repository
DEBIAN_MIRROR ?=

# Docker image build and push setting
DOCKER:=DOCKER_BUILDKIT=1 docker
DOCKERFILE_DIR?=./docker

# BUILDX_PLATFORMS ?= $(subst -,/,$(ARCH))
BUILDX_PLATFORMS ?= linux/amd64,linux/arm64

# Image URL to use all building/pushing image targets
IMG ?= docker.io/apecloud/$(APP_NAME)
TOOL_IMG ?= docker.io/apecloud/$(APP_NAME)-tools
CLI_IMG ?= docker.io/apecloud/kbcli
CHARTS_IMG ?= docker.io/apecloud/$(APP_NAME)-charts
CLI_TAG ?= v$(CLI_VERSION)
DATASCRIPT_IMG ?= docker.io/apecloud/$(APP_NAME)-datascript
DATAPROTECTION_IMG ?= docker.io/apecloud/$(APP_NAME)-dataprotection

# Update whenever you upgrade dev container image
DEV_CONTAINER_VERSION_TAG ?= latest
DEV_CONTAINER_IMAGE_NAME = docker.io/apecloud/$(APP_NAME)-dev

DEV_CONTAINER_DOCKERFILE = Dockerfile-dev
DOCKERFILE_DIR = ./docker
GO_BUILD_ARGS ?= --build-arg GITHUB_PROXY=$(GITHUB_PROXY) --build-arg GOPROXY=$(GOPROXY)
BUILD_ARGS ?= --build-arg VERSION=$(VERSION) --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg GIT_VERSION=$(GIT_VERSION)
DOCKER_BUILD_ARGS ?=
DOCKER_BUILD_ARGS += $(GO_BUILD_ARGS) $(BUILD_ARGS)

##@ Docker containers

.PHONY: build-dev-container-image
build-dev-container-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR)
build-dev-container-image: install-docker-buildx ## Build dev container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/${DEV_CONTAINER_DOCKERFILE} --tag $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	$(DOCKER) buildx build $(DOCKERFILE_DIR)/.  $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) --file $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) --tag $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
endif


.PHONY: push-dev-container-image
push-dev-container-image: DOCKER_BUILD_ARGS += --build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR)
push-dev-container-image: install-docker-buildx ## Push dev container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) push $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG)
else
	$(DOCKER) buildx build  $(DOCKERFILE_DIR)/. $(DOCKER_BUILD_ARGS) --platform $(BUILDX_PLATFORMS) --file $(DOCKERFILE_DIR)/$(DEV_CONTAINER_DOCKERFILE) --tag $(DEV_CONTAINER_IMAGE_NAME):$(DEV_CONTAINER_VERSION_TAG) --push
endif


.PHONY: build-manager-image
build-manager-image: install-docker-buildx generate ## Build Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile --tag ${IMG}:${VERSION} --tag ${IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile --platform $(BUILDX_PLATFORMS) --tag ${IMG}:latest
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile --platform $(BUILDX_PLATFORMS) --tag ${IMG}:${VERSION}
endif
endif


.PHONY: push-manager-image
push-manager-image: install-docker-buildx generate ## Push Operator manager container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	$(DOCKER) push ${IMG}:latest
else
	$(DOCKER) push ${IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile --platform $(BUILDX_PLATFORMS) --tag ${IMG}:latest --push
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile --platform $(BUILDX_PLATFORMS) --tag ${IMG}:${VERSION} --push
endif
endif


.PHONY: build-tools-image
build-tools-image: install-docker-buildx generate test-go-generate ## Build tools container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-tools --tag ${TOOL_IMG}:${VERSION} --tag ${TOOL_IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-tools --platform $(BUILDX_PLATFORMS) --tag ${TOOL_IMG}:latest
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-tools --platform $(BUILDX_PLATFORMS) --tag ${TOOL_IMG}:${VERSION}
endif
endif

.PHONY: push-tools-image
push-tools-image: install-docker-buildx generate test-go-generate ## Push tools container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	$(DOCKER) push ${TOOL_IMG}:latest
else
	$(DOCKER) push ${TOOL_IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-tools --platform $(BUILDX_PLATFORMS) --tag ${TOOL_IMG}:latest --push
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-tools --platform $(BUILDX_PLATFORMS) --tag ${TOOL_IMG}:${VERSION} --push
endif
endif

.PHONY: build-charts-image
build-charts-image: install-docker-buildx helm-package ## Build helm charts container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-charts --tag ${CHARTS_IMG}:${VERSION} --tag ${CHARTS_IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-charts --platform $(BUILDX_PLATFORMS) --tag ${CHARTS_IMG}:latest
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-charts --platform $(BUILDX_PLATFORMS) --tag ${CHARTS_IMG}:${VERSION}
endif
endif


.PHONY: push-charts-image
push-charts-image: install-docker-buildx helm-package ## Push helm charts container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	$(DOCKER) push ${CHARTS_IMG}:latest
else
	$(DOCKER) push ${CHARTS_IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-charts --platform $(BUILDX_PLATFORMS) --tag ${CHARTS_IMG}:latest --push
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-charts --platform $(BUILDX_PLATFORMS) --tag ${CHARTS_IMG}:${VERSION} --push
endif
endif

.PHONY: build-datascript-image
build-datascript-image: install-docker-buildx ## Build datascript container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-datascript --tag ${DATASCRIPT_IMG}:${VERSION} --tag ${DATASCRIPT_IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-datascript --platform $(BUILDX_PLATFORMS) --tag ${DATASCRIPT_IMG}:latest
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-datascript --platform $(BUILDX_PLATFORMS) --tag ${DATASCRIPT_IMG}:${VERSION}
endif
endif

.PHONY: push-datascript-image
push-datascript-image: install-docker-buildx  ## Push datascript container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	$(DOCKER) push ${DATASCRIPT_IMG}:latest
else
	$(DOCKER) push ${DATASCRIPT_IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-datascript --platform $(BUILDX_PLATFORMS) --tag ${DATASCRIPT_IMG}:latest --push
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-datascript --platform $(BUILDX_PLATFORMS) --tag ${DATASCRIPT_IMG}:${VERSION} --push
endif
endif

.PHONY: build-dataprotection-image
build-dataprotection-image: install-docker-buildx generate ## Build Operator dataprotection container image.
ifneq ($(BUILDX_ENABLED), true)
	$(DOCKER) build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-dataprotection --tag ${DATAPROTECTION_IMG}:${VERSION} --tag ${DATAPROTECTION_IMG}:latest
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-dataprotection --platform $(BUILDX_PLATFORMS) --tag ${DATAPROTECTION_IMG}:latest
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-dataprotection --platform $(BUILDX_PLATFORMS) --tag ${DATAPROTECTION_IMG}:${VERSION}
endif
endif

.PHONY: push-dataprotection-image
push-dataprotection-image: install-docker-buildx generate ## Push Operator dataprotection container image.
ifneq ($(BUILDX_ENABLED), true)
ifeq ($(TAG_LATEST), true)
	$(DOCKER) push ${DATAPROTECTION_IMG}:latest
else
	$(DOCKER) push ${DATAPROTECTION_IMG}:${VERSION}
endif
else
ifeq ($(TAG_LATEST), true)
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-dataprotection --platform $(BUILDX_PLATFORMS) --tag ${DATAPROTECTION_IMG}:latest --push
else
	$(DOCKER) buildx build . $(DOCKER_BUILD_ARGS) --file $(DOCKERFILE_DIR)/Dockerfile-dataprotection --platform $(BUILDX_PLATFORMS) --tag ${DATAPROTECTION_IMG}:${VERSION} --push
endif
endif