#!/usr/bin/env bash
#
# Copyright (C) 2022-2026 ApeCloud Co., Ltd
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

GENERATORS="client,informer,lister"
OUTPUT_PACKAGE="github.com/apecloud/kubeblocks/pkg/client"
APIS_PACKAGE="github.com/apecloud/kubeblocks/apis"
GROUP_VERSIONS="apps:v1alpha1 apps:v1beta1 apps:v1 dataprotection:v1alpha1 extensions:v1alpha1 operations:v1alpha1 workloads:v1 parameters:v1alpha1"
OUTPUT_BASE="${SCRIPT_ROOT}/hack"

trap 'rm -rf "${OUTPUT_BASE}/github.com"' EXIT

(
  cd "${CODE_GENERATOR_PATH}"
  GO111MODULE=on go install \
    k8s.io/code-generator/cmd/client-gen \
    k8s.io/code-generator/cmd/informer-gen \
    k8s.io/code-generator/cmd/lister-gen
)

GOBIN="$(go env GOBIN)"
GOBIN="${GOBIN:-$(go env GOPATH)/bin}"

function join_by() {
  local IFS="$1"
  shift
  echo "$*"
}

FQ_APIS=()
for gv in ${GROUP_VERSIONS}; do
  IFS=: read -r group versions <<<"${gv}"
  for version in ${versions//,/ }; do
    FQ_APIS+=("${APIS_PACKAGE}/${group}/${version}")
  done
done

INPUT_DIRS="$(join_by , "${FQ_APIS[@]}")"
GO_HEADER_FILE="${SCRIPT_ROOT}/hack/boilerplate_apache2.go.txt"

"${GOBIN}/client-gen" \
  --clientset-name versioned \
  --input-base "" \
  --input "${INPUT_DIRS}" \
  --output-package "${OUTPUT_PACKAGE}/clientset" \
  --go-header-file "${GO_HEADER_FILE}" \
  --output-base "${OUTPUT_BASE}"

"${GOBIN}/lister-gen" \
  --input-dirs "${INPUT_DIRS}" \
  --output-package "${OUTPUT_PACKAGE}/listers" \
  --go-header-file "${GO_HEADER_FILE}" \
  --output-base "${OUTPUT_BASE}"

"${GOBIN}/informer-gen" \
  --input-dirs "${INPUT_DIRS}" \
  --versioned-clientset-package "${OUTPUT_PACKAGE}/clientset/versioned" \
  --listers-package "${OUTPUT_PACKAGE}/listers" \
  --output-package "${OUTPUT_PACKAGE}/informers" \
  --go-header-file "${GO_HEADER_FILE}" \
  --output-base "${OUTPUT_BASE}"

rm -rf "${SCRIPT_ROOT}/pkg/client"
mv "${OUTPUT_BASE}/${OUTPUT_PACKAGE}" "${SCRIPT_ROOT}/pkg/client"
rm -rf "${OUTPUT_BASE}/github.com"
