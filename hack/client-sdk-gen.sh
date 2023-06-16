#!/usr/bin/env bash
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
#along with this program.  If not, see <http://www.gnu.org/licenses/>


set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname $0)/../" && pwd -P)"

if [ -d "${SCRIPT_ROOT}/vendor" ]; then
  export GOFLAGS="-mod=readonly"
fi

CODE_GENERATOR_PATH=$(go list -f '{{.Dir}}' -m k8s.io/code-generator)

GENERATORS="all"   # deepcopy,defaulter,client,lister,informer or all
OUTPUT_PACKAGE="github.com/apecloud/kubeblocks/pkg/client"
APIS_PACKAGE="github.com/apecloud/kubeblocks/apis"
GROUP_VERSIONS="apps:v1alpha1 dataprotection:v1alpha1 extensions:v1alpha1 workloads:v1alpha1"
OUTPUT_BASE="${SCRIPT_ROOT}/hack"


bash ${CODE_GENERATOR_PATH}/generate-groups.sh \
  ${GENERATORS} \
  "${OUTPUT_PACKAGE}" \
  "${APIS_PACKAGE}" \
  "${GROUP_VERSIONS}" \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate_apache2.go.txt" \
  --output-base "${OUTPUT_BASE}"

rm -rf "${SCRIPT_ROOT}/pkg/client"
mv "${OUTPUT_BASE}/${OUTPUT_PACKAGE}" "${SCRIPT_ROOT}/pkg/client"
rm -rf "${OUTPUT_BASE}/github.com"

