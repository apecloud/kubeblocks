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
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CONFIG_RENDER_TOOL_LD_FLAGS} -o $@ ./cmd/reloader/template/main.go

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

## probe cmd

PROBE_LD_FLAGS = "-s -w"

bin/probe.%: ## Cross build bin/probe.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${PROBE_LD_FLAGS} -o $@  ./cmd/probe/main.go

.PHONY: probe
probe: OS=$(shell $(GO) env GOOS)
probe: ARCH=$(shell $(GO) env GOARCH)
probe: test-go-generate build-checks ## Build probe related binaries
	$(MAKE) bin/probe.${OS}.${ARCH}
	mv bin/probe.${OS}.${ARCH} bin/probe

.PHONY: probe-fast
probe-fast: OS=$(shell $(GO) env GOOS)
probe-fast: ARCH=$(shell $(GO) env GOARCH)
probe-fast:
	$(MAKE) bin/probe.${OS}.${ARCH}
	mv bin/probe.${OS}.${ARCH} bin/probe

.PHONY: clean-probe
clean-probe: ## Clean bin/probe.
	rm -f bin/probe
