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

##@ Sub-commands

## reloader cmd

RELOADER_LD_FLAGS = "-s -w"

bin/reloader.%: ## Cross build bin/reloader.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${RELOADER_LD_FLAGS} -o $@ ./cmd/reloader/main.go

.PHONY: reloader
reloader: OS=$(shell $(GO) env GOOS)
reloader: ARCH=$(shell $(GO) env GOARCH)
reloader: test-go-generate build-checks ## Build reloader related binaries
	$(MAKE) bin/reloader.${OS}.${ARCH}
	mv bin/reloader.${OS}.${ARCH} bin/reloader

.PHONY: clean-reloader
clean-reloader: ## Clean bin/reloader.
	rm -f bin/reloader

## config_render cmd

CONFIG_RENDER_TOOL_LD_FLAGS = "-s -w"

bin/config_render.%: ## Cross build bin/config_render.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CONFIG_RENDER_TOOL_LD_FLAGS} -o $@ ./cmd/reloader/template/*.go

.PHONY: config_render
config_render: OS=$(shell $(GO) env GOOS)
config_render: ARCH=$(shell $(GO) env GOARCH)
config_render: build-checks ## Build config_render related binaries
	$(MAKE) bin/config_render.${OS}.${ARCH}
	mv bin/config_render.${OS}.${ARCH} bin/config_render

.PHONY: clean-config_render
clean-config_render: ## Clean bin/tpltool.
	rm -f bin/config_render

## cue-helper cmd

CUE_HELPER_LD_FLAGS = "-s -w"

bin/cue-helper.%: ## Cross build bin/cue-helper.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CUE_HELPER_LD_FLAGS} -o $@ ./cmd/reloader/tools/main.go

.PHONY: cue-helper
cue-helper: OS=$(shell $(GO) env GOOS)
cue-helper: ARCH=$(shell $(GO) env GOARCH)
cue-helper: test-go-generate build-checks ## Build cue-helper related binaries
	$(MAKE) bin/cue-helper.${OS}.${ARCH}
	mv bin/cue-helper.${OS}.${ARCH} bin/cue-helper

.PHONY: clean-cue-helper
clean-cue-helper: ## Clean bin/cue-helper.
	rm -f bin/cue-helper

## lorry cmd

LORRY_LD_FLAGS = "-s -w"

bin/lorry.%: ## Cross build bin/lorry.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${LORRY_LD_FLAGS} -o $@  ./cmd/lorry/main.go

.PHONY: lorry
lorry: OS=$(shell $(GO) env GOOS)
lorry: ARCH=$(shell $(GO) env GOARCH)
lorry: test-go-generate build-checks ## Build lorry related binaries
	$(MAKE) bin/lorry.${OS}.${ARCH}
	mv bin/lorry.${OS}.${ARCH} bin/lorry

.PHONY: lorry-fast
lorry-fast: OS=$(shell $(GO) env GOOS)
lorry-fast: ARCH=$(shell $(GO) env GOARCH)
lorry-fast:
	$(MAKE) bin/lorry.${OS}.${ARCH}
	mv bin/lorry.${OS}.${ARCH} bin/lorry

.PHONY: clean-lorry
clean-lorry: ## Clean bin/lorry.
	rm -f bin/lorry
