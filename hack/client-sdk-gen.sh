#!/usr/bin/env bash
#
# Copyright (C) 2022-2024 ApeCloud Co., Ltd
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname $0)/../" && pwd -P)"

if [ -d "${SCRIPT_ROOT}/vendor" ]; then
  export GOFLAGS="-mod=readonly"
fi

CODE_GENERATOR_PATH=$(go list -f '{{.Dir}}' -m k8s.io/code-generator)

# HACK: add exec permission to code generator scripts
chmod u+x ${CODE_GENERATOR_PATH}/*.sh

GENERATORS="client,informer,lister"
APIS_PACKAGE="github.com/apecloud/kubeblocks/apis"
OUTPUT_PACKAGE="github.com/apecloud/kubeblocks/pkg/client"
GROUP_VERSIONS="apps:v1alpha1 apps:v1beta1 apps:v1 dataprotection:v1alpha1 extensions:v1alpha1 workloads:v1alpha1"
OUTPUT_BASE="${SCRIPT_ROOT}/hack"


bash ${CODE_GENERATOR_PATH}/generate-groups.sh \
  ${GENERATORS} \
  "${OUTPUT_PACKAGE}" \
  "${APIS_PACKAGE}" \
  "${GROUP_VERSIONS}" \
  --output-base "${OUTPUT_BASE}" \
  --go-header-file "${SCRIPT_ROOT}/hack/boilerplate_apache2.go.txt"

rm -rf "${SCRIPT_ROOT}/pkg/client"
mv "${OUTPUT_BASE}/${OUTPUT_PACKAGE}" "${SCRIPT_ROOT}/pkg/client"
rm -rf "${OUTPUT_BASE}/github.com"
