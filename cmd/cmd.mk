#
# Copyright ApeCloud, Inc.
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

## ctool cmd

CONFIG_TOOL_LD_FLAGS = "-s -w"

bin/ctool.%: ## Cross build bin/ctool.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CONFIG_TOOL_LD_FLAGS} -o $@ ./cmd/tpl/main.go

.PHONY: ctool
ctool: OS=$(shell $(GO) env GOOS)
ctool: ARCH=$(shell $(GO) env GOARCH)
ctool: build-checks ## Build ctool related binaries
	$(MAKE) bin/ctool.${OS}.${ARCH}
	mv bin/ctool.${OS}.${ARCH} bin/ctool

.PHONY: clean-ctool
clean-ctool: ## Clean bin/ctool.
	rm -f bin/ctool

## cue-helper cmd

CUE_HELPER_LD_FLAGS = "-s -w"

bin/cue-helper.%: ## Cross build bin/cue-helper.$(OS).$(ARCH) .
	GOOS=$(word 2,$(subst ., ,$@)) GOARCH=$(word 3,$(subst ., ,$@)) $(GO) build -ldflags=${CUE_HELPER_LD_FLAGS} -o $@ ./cmd/reloader/tools/cue_auto_generator.go

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
