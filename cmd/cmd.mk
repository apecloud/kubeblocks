#
#Copyright (C) 2022-2025 ApeCloud Co., Ltd
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