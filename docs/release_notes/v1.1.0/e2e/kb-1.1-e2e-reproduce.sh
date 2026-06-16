#!/usr/bin/env bash

set -euo pipefail

# Reproduce focused KubeBlocks 1.1 release validation cases:
#   upgrade from KB 1.0.3, ComponentNetwork, Rollout API,
#   dynamic instance adoption, and sharding tests.
#
# The script intentionally uses kubectl create/replace instead of kubectl apply
# so CRD and CR updates exercise the same paths used during release testing.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"
MANIFEST_DIR="${MANIFEST_DIR:-${SCRIPT_DIR}}"
WORK_DIR="${WORK_DIR:-./docs/release_notes/v1.1.0/e2e/}"

KB_NAMESPACE="${KB_NAMESPACE:-kb-system}"
TEST_NAMESPACE="${TEST_NAMESPACE:-kb-11-e2e}"
KIND_CLUSTER="${KIND_CLUSTER:-kb-upgrade-103b9}"
SOURCE_VERSION="${SOURCE_VERSION:-1.0.3-beta.9}"
TARGET_VERSION="${TARGET_VERSION:-1.1.0-beta.6}"
AUTO_INSTALLED_ADDONS="${AUTO_INSTALLED_ADDONS:-[\"mysql\",\"redis\",\"mongodb\"]}"

CREATE_KIND="${CREATE_KIND:-true}"
RESET_KIND="${RESET_KIND:-false}"
RESET_TEST_NAMESPACE="${RESET_TEST_NAMESPACE:-false}"
SKIP_HELM_REPO_UPDATE="${SKIP_HELM_REPO_UPDATE:-false}"

MYSQL_CLUSTER="${MYSQL_CLUSTER:-mysql-cluster}"
MYSQL_COMPDEF="${MYSQL_COMPDEF:-mysql-8.0-1.0.5}"
MYSQL_VERSION_BASE="${MYSQL_VERSION_BASE:-8.0.35}"
MYSQL_VERSION_INPLACE="${MYSQL_VERSION_INPLACE:-8.0.36}"
MYSQL_VERSION_REPLACE="${MYSQL_VERSION_REPLACE:-8.0.37}"
MYSQL_VERSION_CANARY="${MYSQL_VERSION_CANARY:-8.0.38}"
ROLLOUT_INSTANCE_INTERVAL_SECONDS="${ROLLOUT_INSTANCE_INTERVAL_SECONDS:-30}"
ROLLOUT_SCALE_DOWN_DELAY_SECONDS="${ROLLOUT_SCALE_DOWN_DELAY_SECONDS:-30}"

MONGO_COMPDEF="${MONGO_COMPDEF:-mongodb-1.1.0-alpha.0}"
MONGO_VERSION="${MONGO_VERSION:-6.0.27}"
MONGO_SHARD_STORAGE="${MONGO_SHARD_STORAGE:-10Gi}"
MONGO_NETWORK_STORAGE_CLASS="${MONGO_NETWORK_STORAGE_CLASS:-}"

REDIS_COMPDEF="${REDIS_COMPDEF:-redis-cluster-7-1.1.0-alpha.0}"
REDIS_VERSION_BASE="${REDIS_VERSION_BASE:-7.2.4}"
REDIS_VERSION_TARGET="${REDIS_VERSION_TARGET:-7.2.5}"

log() {
  printf '\n[%s] %s\n' "$(date '+%H:%M:%S')" "$*"
}

fail() {
  printf '\nERROR: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'USAGE'
Usage:
  bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh [stage ...]

Setup stages:
  setup-kind                    Create or select the kind cluster.
  setup-kb                      Install or upgrade KB TARGET_VERSION for non-upgrade cases.
  cleanup                       Delete test namespace.

Test case stages:
  case-upgrade                    Upgrade KB 1.0.3 to KB 1.1 and check MySQL pod/PVC stability.
  case-network                    ComponentNetwork DNS, hostPorts, and hostNetwork.
  case-rollout-api                Rollout API for MySQL and sharding targets.
  case-dynamic-instance-template  Dynamic InstanceSet template adoption.
  case-sharding                   Sharding lifecycle, offline shard, and heterogeneous shards.

Useful env vars:
  SOURCE_VERSION=1.0.3-beta.9
  TARGET_VERSION=1.1.0-beta.6
  KIND_CLUSTER=kb-upgrade-103b9
  CREATE_KIND=true|false
  RESET_KIND=true|false
  RESET_TEST_NAMESPACE=true|false
  AUTO_INSTALLED_ADDONS='["mysql","redis","mongodb"]'
  MONGO_NETWORK_STORAGE_CLASS=''
  ROLLOUT_INSTANCE_INTERVAL_SECONDS=30
  ROLLOUT_SCALE_DOWN_DELAY_SECONDS=30
  MANIFEST_DIR=docs/release_notes/v1.1.0/e2e
  WORK_DIR=./docs/release_notes/v1.1.0/e2e/

Examples:
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-kb
  RESET_KIND=true bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-upgrade
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-network
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-rollout-api case-dynamic-instance-template
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

prepare_dirs() {
  mkdir -p "${WORK_DIR}"
}

preflight() {
  require_cmd kubectl
  require_cmd jq
  require_cmd perl
  prepare_dirs
  log "repo root: ${ROOT_DIR}"
  log "manifest dir: ${MANIFEST_DIR}"
  log "work dir: ${WORK_DIR}"
}

manifest_path() {
  local name="$1"
  printf '%s/%s' "${MANIFEST_DIR}" "${name}"
}

render_manifest() {
  local manifest="$1"
  shift

  local -a env_args=(
    "KB_NAMESPACE=${KB_NAMESPACE}"
    "TEST_NAMESPACE=${TEST_NAMESPACE}"
    "MYSQL_CLUSTER=${MYSQL_CLUSTER}"
    "MYSQL_COMPDEF=${MYSQL_COMPDEF}"
    "MYSQL_VERSION_BASE=${MYSQL_VERSION_BASE}"
    "MYSQL_VERSION_INPLACE=${MYSQL_VERSION_INPLACE}"
    "MYSQL_VERSION_REPLACE=${MYSQL_VERSION_REPLACE}"
    "MYSQL_VERSION_CANARY=${MYSQL_VERSION_CANARY}"
    "ROLLOUT_INSTANCE_INTERVAL_SECONDS=${ROLLOUT_INSTANCE_INTERVAL_SECONDS}"
    "ROLLOUT_SCALE_DOWN_DELAY_SECONDS=${ROLLOUT_SCALE_DOWN_DELAY_SECONDS}"
    "MONGO_COMPDEF=${MONGO_COMPDEF}"
    "MONGO_VERSION=${MONGO_VERSION}"
    "MONGO_SHARD_STORAGE=${MONGO_SHARD_STORAGE}"
    "MONGO_NETWORK_STORAGE_CLASS=${MONGO_NETWORK_STORAGE_CLASS}"
    "REDIS_COMPDEF=${REDIS_COMPDEF}"
    "REDIS_VERSION_BASE=${REDIS_VERSION_BASE}"
    "REDIS_VERSION_TARGET=${REDIS_VERSION_TARGET}"
  )

  [[ $(($# % 2)) -eq 0 ]] || fail "render_manifest extra variables must be KEY VALUE pairs"

  while [[ "$#" -gt 0 ]]; do
    env_args+=("$1=$2")
    shift 2
  done

  env "${env_args[@]}" perl -pe 's/\$\{([A-Za-z_][A-Za-z0-9_]*)\}/exists $ENV{$1} ? $ENV{$1} : $&/ge' "${manifest}"
}

create_manifest() {
  local name="$1"
  shift

  local manifest
  manifest="$(manifest_path "${name}")"
  [[ -f "${manifest}" ]] || fail "missing manifest: ${manifest}"
  render_manifest "${manifest}" "$@" | kubectl create -f -
}

patch_cluster_json() {
  local cluster="$1"
  local name="$2"
  shift 2

  local manifest
  manifest="$(manifest_path "${name}")"
  [[ -f "${manifest}" ]] || fail "missing manifest: ${manifest}"
  render_manifest "${manifest}" "$@" \
    | kubectl patch cluster "${cluster}" -n "${TEST_NAMESPACE}" --type json --patch-file /dev/stdin
}

kind_context() {
  printf 'kind-%s' "${KIND_CLUSTER}"
}

create_or_select_kind() {
  if [[ "${CREATE_KIND}" != "true" ]]; then
    log "CREATE_KIND=false, using current kubectl context: $(kubectl config current-context)"
    return
  fi
  require_cmd kind

  if kind get clusters | grep -qx "${KIND_CLUSTER}"; then
    if [[ "${RESET_KIND}" == "true" ]]; then
      log "deleting existing kind cluster ${KIND_CLUSTER}"
      kind delete cluster --name "${KIND_CLUSTER}"
    else
      log "kind cluster ${KIND_CLUSTER} already exists"
    fi
  fi

  if ! kind get clusters | grep -qx "${KIND_CLUSTER}"; then
    log "creating kind cluster ${KIND_CLUSTER} without inherited proxy env"
    env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy \
      kind create cluster --name "${KIND_CLUSTER}" --config "$(manifest_path kind-three-node.yaml)"
  fi

  kubectl config use-context "$(kind_context)"
  kubectl cluster-info
}

ensure_namespace() {
  local ns="$1"
  if kubectl get namespace "${ns}" >/dev/null 2>&1; then
    log "namespace ${ns} already exists"
  else
    kubectl create namespace "${ns}"
  fi
}

reset_test_namespace_if_requested() {
  if [[ "${RESET_TEST_NAMESPACE}" != "true" ]]; then
    ensure_namespace "${TEST_NAMESPACE}"
    return
  fi

  if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
    log "deleting test namespace ${TEST_NAMESPACE}"
    kubectl delete namespace "${TEST_NAMESPACE}" --wait=true
  fi
  kubectl create namespace "${TEST_NAMESPACE}"
}

wait_deploy_if_exists() {
  local ns="$1"
  local deploy="$2"
  if kubectl -n "${ns}" get deploy "${deploy}" >/dev/null 2>&1; then
    kubectl -n "${ns}" rollout status "deploy/${deploy}" --timeout=10m
  else
    log "deployment ${ns}/${deploy} does not exist, skipping rollout wait"
  fi
}

wait_component_definition() {
  local name="$1"
  log "waiting for ComponentDefinition ${name}"
  for _ in $(seq 1 120); do
    if kubectl get componentdefinition "${name}" >/dev/null 2>&1; then
      return
    fi
    sleep 5
  done
  fail "ComponentDefinition ${name} did not appear"
}

ensure_helm_repo() {
  require_cmd helm
  if ! helm repo list -o json | jq -e '.[] | select(.name == "kubeblocks")' >/dev/null; then
    helm repo add kubeblocks https://apecloud.github.io/helm-charts
  fi
  if [[ "${SKIP_HELM_REPO_UPDATE}" != "true" ]]; then
    helm repo update kubeblocks
  fi
}

install_snapshot_crds() {
  log "installing Snapshot CRDs before Helm"
  if kubectl get crd volumesnapshots.snapshot.storage.k8s.io >/dev/null 2>&1; then
    log "Snapshot CRDs already exist"
  else
    kubectl create -f "${ROOT_DIR}/deploy/helm/crds/snapshot/"
  fi
}

install_source_kubeblocks() {
  ensure_helm_repo
  ensure_namespace "${KB_NAMESPACE}"
  install_snapshot_crds

  log "installing KubeBlocks CRDs for v${SOURCE_VERSION}"
  if kubectl get crd clusters.apps.kubeblocks.io >/dev/null 2>&1; then
    log "KubeBlocks CRDs already exist; source CRD create is skipped"
  else
    kubectl create -f "https://github.com/apecloud/kubeblocks/releases/download/v${SOURCE_VERSION}/kubeblocks_crds.yaml"
  fi

  if helm -n "${KB_NAMESPACE}" status kubeblocks >/dev/null 2>&1; then
    log "Helm release kubeblocks already exists in ${KB_NAMESPACE}; install-source is skipped"
  else
    log "installing kubeblocks chart ${SOURCE_VERSION}"
    helm install kubeblocks kubeblocks/kubeblocks \
      --version "${SOURCE_VERSION}" \
      -n "${KB_NAMESPACE}" \
      --create-namespace \
      --skip-crds \
      --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"
  fi

  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks-dataprotection
  wait_component_definition "${MYSQL_COMPDEF}"
}

install_target_kubeblocks() {
  ensure_helm_repo
  ensure_namespace "${KB_NAMESPACE}"
  install_snapshot_crds

  log "installing or replacing KubeBlocks CRDs for v${TARGET_VERSION}"
  if kubectl get crd clusters.apps.kubeblocks.io >/dev/null 2>&1; then
    kubectl replace -f "https://github.com/apecloud/kubeblocks/releases/download/v${TARGET_VERSION}/kubeblocks_crds.yaml"
  else
    kubectl create -f "https://github.com/apecloud/kubeblocks/releases/download/v${TARGET_VERSION}/kubeblocks_crds.yaml"
  fi

  if helm -n "${KB_NAMESPACE}" status kubeblocks >/dev/null 2>&1; then
    log "upgrading kubeblocks chart to ${TARGET_VERSION}"
    helm -n "${KB_NAMESPACE}" upgrade kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      --skip-crds \
      --reuse-values \
      --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"
  else
    log "installing kubeblocks chart ${TARGET_VERSION}"
    helm install kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      -n "${KB_NAMESPACE}" \
      --create-namespace \
      --skip-crds \
      --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"
  fi

  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks-dataprotection
  wait_component_definition "${MYSQL_COMPDEF}"
}

wait_cluster_running() {
  local name="$1"
  local ns="${2:-${TEST_NAMESPACE}}"
  kubectl wait "cluster/${name}" -n "${ns}" \
    --for=jsonpath='{.status.phase}'=Running \
    --timeout=30m
}

wait_rollout_succeed() {
  local name="$1"
  local ns="${2:-${TEST_NAMESPACE}}"
  kubectl wait "rollout/${name}" -n "${ns}" \
    --for=jsonpath='{.status.state}'=Succeed \
    --timeout=35m
}

snapshot_cluster_pods() {
  local name="$1"
  local out="$2"
  kubectl get pod -n "${TEST_NAMESPACE}" \
    -l "app.kubernetes.io/instance=${name}" \
    -o json \
    | jq '[.items[] | {name: .metadata.name, uid: .metadata.uid, created: .metadata.creationTimestamp, readyTime: ([.status.conditions[]? | select(.type == "Ready" and .status == "True") | .lastTransitionTime][0] // ""), phase: .status.phase, template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"] // ""), serviceVersion: (.metadata.labels["apps.kubeblocks.io/service-version"] // ""), images: [.spec.containers[] | {name, image}]}] | sort_by(.name)' \
    >"${out}"
}

assert_min_ready_interval() {
  local pod_snapshot="$1"
  local min_seconds="$2"
  jq -e --argjson min_seconds "${min_seconds}" '
    [.[] | select(.readyTime != "") | .readyTime | fromdateiso8601] | sort
    | if length < 2 then true
      else [range(1; length) as $i | (.[$i] - .[$i - 1]) >= $min_seconds] | all
      end
  ' "${pod_snapshot}" >/dev/null
}

snapshot_all_managed_pods() {
  local out="$1"
  kubectl get pod -A -l app.kubernetes.io/managed-by=kubeblocks -o json \
    | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' \
    >"${out}"
}

snapshot_all_pvcs() {
  local out="$1"
  kubectl get pvc -A -o json \
    | jq '[.items[] | {namespace: .metadata.namespace, name: .metadata.name, uid: .metadata.uid}] | sort_by(.namespace, .name)' \
    >"${out}"
}

create_mysql_test_cluster() {
  reset_test_namespace_if_requested

  if kubectl -n "${TEST_NAMESPACE}" get cluster "${MYSQL_CLUSTER}" >/dev/null 2>&1; then
    log "cluster ${MYSQL_CLUSTER} already exists; skipping create"
  else
    create_manifest mysql-rollout-base.yaml
  fi

  wait_cluster_running "${MYSQL_CLUSTER}"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\n"}'
}

create_mysql_base_cluster() {
  create_mysql_test_cluster
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-before-upgrade-pods.json"
  snapshot_all_managed_pods "${WORK_DIR}/before-upgrade-managed-pods.json"
  snapshot_all_pvcs "${WORK_DIR}/before-upgrade-pvcs.json"
}

upgrade_to_target_kubeblocks() {
  ensure_helm_repo

  log "creating new 1.1 CRDs if they do not exist"
  kubectl get crd rollouts.apps.kubeblocks.io >/dev/null 2>&1 \
    || kubectl create -f "${ROOT_DIR}/deploy/helm/crds/apps.kubeblocks.io_rollouts.yaml"
  kubectl get crd instances.workloads.kubeblocks.io >/dev/null 2>&1 \
    || kubectl create -f "${ROOT_DIR}/deploy/helm/crds/workloads.kubeblocks.io_instances.yaml"
  kubectl get crd instancesets.workloads.kubeblocks.io >/dev/null 2>&1 \
    || kubectl create -f "${ROOT_DIR}/deploy/helm/crds/workloads.kubeblocks.io_instancesets.yaml"

  log "replacing CRDs with v${TARGET_VERSION} release asset"
  kubectl replace -f "https://github.com/apecloud/kubeblocks/releases/download/v${TARGET_VERSION}/kubeblocks_crds.yaml"

  log "upgrading Helm release to ${TARGET_VERSION}"
  helm -n "${KB_NAMESPACE}" upgrade kubeblocks kubeblocks/kubeblocks \
    --version "${TARGET_VERSION}" \
    --skip-crds \
    --reuse-values \
    --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"

  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks-dataprotection
  kubectl get crd clusters.apps.kubeblocks.io rollouts.apps.kubeblocks.io \
    instances.workloads.kubeblocks.io instancesets.workloads.kubeblocks.io \
    volumesnapshots.snapshot.storage.k8s.io

  wait_cluster_running "${MYSQL_CLUSTER}"
  snapshot_all_managed_pods "${WORK_DIR}/after-upgrade-managed-pods.json"
  snapshot_all_pvcs "${WORK_DIR}/after-upgrade-pvcs.json"

  log "checking database pod UID stability across operator upgrade"
  if ! diff -u "${WORK_DIR}/before-upgrade-managed-pods.json" "${WORK_DIR}/after-upgrade-managed-pods.json"; then
    log "pod UID diff detected; inspect ${WORK_DIR}/before-upgrade-managed-pods.json and ${WORK_DIR}/after-upgrade-managed-pods.json"
  fi

  log "checking PVC UID stability across operator upgrade"
  if ! diff -u "${WORK_DIR}/before-upgrade-pvcs.json" "${WORK_DIR}/after-upgrade-pvcs.json"; then
    log "PVC UID diff detected; inspect ${WORK_DIR}/before-upgrade-pvcs.json and ${WORK_DIR}/after-upgrade-pvcs.json"
  fi
}

delete_rollout_if_exists() {
  local name="$1"
  if kubectl -n "${TEST_NAMESPACE}" get rollout "${name}" >/dev/null 2>&1; then
    kubectl delete rollout "${name}" -n "${TEST_NAMESPACE}" --wait=true
  fi
}

run_mysql_inplace_rollout() {
  delete_rollout_if_exists mysql-inplace-8036
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-before-inplace.json"

  create_manifest mysql-inplace-8036.yaml
  wait_rollout_succeed mysql-inplace-8036
  wait_cluster_running "${MYSQL_CLUSTER}"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-after-inplace.json"

  log "inplace rollout should keep pod names and UIDs stable"
  diff -u \
    <(jq '[.[] | {name, uid}]' "${WORK_DIR}/mysql-before-inplace.json") \
    <(jq '[.[] | {name, uid}]' "${WORK_DIR}/mysql-after-inplace.json")
  kubectl get rollout mysql-inplace-8036 -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-inplace-8036.result.yaml"
  delete_rollout_if_exists mysql-inplace-8036
}

run_mysql_replace_rollout() {
  delete_rollout_if_exists mysql-replace-8037
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-before-replace.json"

  create_manifest mysql-replace-8037.yaml
  wait_rollout_succeed mysql-replace-8037
  wait_cluster_running "${MYSQL_CLUSTER}"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-after-replace.json"

  log "replace rollout is expected to recreate stable pods"
  log "replace rollout uses ${ROLLOUT_INSTANCE_INTERVAL_SECONDS}s per-instance interval and ${ROLLOUT_SCALE_DOWN_DELAY_SECONDS}s scale-down delay"
  assert_min_ready_interval "${WORK_DIR}/mysql-after-replace.json" "${ROLLOUT_INSTANCE_INTERVAL_SECONDS}"
  diff -u "${WORK_DIR}/mysql-before-replace.json" "${WORK_DIR}/mysql-after-replace.json" || true
  kubectl get rollout mysql-replace-8037 -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-replace-8037.result.yaml"
  delete_rollout_if_exists mysql-replace-8037
}

run_mysql_create_rollout() {
  delete_rollout_if_exists mysql-create-8038

  create_manifest mysql-create-8038.yaml
  wait_rollout_succeed mysql-create-8038
  wait_cluster_running "${MYSQL_CLUSTER}"
  kubectl get rollout mysql-create-8038 -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-create-8038.result.yaml"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-after-create-rollout.cluster.yaml"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-after-create-rollout-pods.json"
  delete_rollout_if_exists mysql-create-8038
}

run_dynamic_hot_instance_test() {
  wait_cluster_running "${MYSQL_CLUSTER}"
  local hot_template
  hot_template="$(
    kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
      | jq -r --arg version "${MYSQL_VERSION_CANARY}" '.spec.componentSpecs[] | select(.name == "mysql") | .instances[]? | select(.serviceVersion == $version) | .name' \
      | head -1
  )"
  [[ -n "${hot_template}" ]] || fail "no MySQL instance template with serviceVersion ${MYSQL_VERSION_CANARY}; run case-rollout-api first"

  log "marking MySQL instance template ${hot_template} as hot"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-hot-before.yaml"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
    | jq --arg name "${hot_template}" '
        (.spec.componentSpecs[] | select(.name == "mysql").instances[] | select(.name == $name).labels) = {"workload-tier":"hot"}
        | (.spec.componentSpecs[] | select(.name == "mysql").instances[] | select(.name == $name).resources) = {
            "requests": {"cpu":"750m","memory":"768Mi"},
            "limits": {"cpu":"750m","memory":"768Mi"}
          }' \
    | kubectl replace -f -

  wait_cluster_running "${MYSQL_CLUSTER}"
  kubectl get pod -n "${TEST_NAMESPACE}" -l workload-tier=hot --show-labels -o wide
  kubectl get pod -n "${TEST_NAMESPACE}" -l workload-tier=hot -o json \
    | jq '.items[] | {name: .metadata.name, uid: .metadata.uid, serviceVersionLabel: .metadata.labels["apps.kubeblocks.io/service-version"], template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"]), images: [.spec.containers[] | {name, image, resources}]}' \
    >"${WORK_DIR}/mysql-hot-pods.json"
}

create_mongodb_sharding_cluster() {
  wait_component_definition mongodb || true
  if kubectl -n "${TEST_NAMESPACE}" get cluster mongodb-sharding >/dev/null 2>&1; then
    log "cluster mongodb-sharding already exists; skipping create"
  else
    create_manifest mongodb-sharding.yaml
  fi

  wait_cluster_running mongodb-sharding
  kubectl get cluster mongodb-sharding -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MONGO_VERSION}" '
        ([.spec.componentSpecs[]?.replicas, .spec.shardings[]?.template.replicas] | all(. != null and . > 0))
        and
        ([.spec.componentSpecs[]?.serviceVersion, .spec.shardings[]?.template.serviceVersion] | all(. == $version))
      ' >/dev/null
  kubectl get components -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=mongodb-sharding -o wide
}

run_mongodb_sharding_lifecycle() {
  create_mongodb_sharding_cluster

  patch_cluster_json mongodb-sharding mongodb-sharding-scale-out-patch.yaml
  wait_cluster_running mongodb-sharding
  wait_shard_component_count mongodb-sharding shard 4
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongodb-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-sharding-after-scale-out.components.yaml"

  patch_cluster_json mongodb-sharding mongodb-sharding-scale-in-patch.yaml
  wait_cluster_running mongodb-sharding
  wait_shard_component_count mongodb-sharding shard 3
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongodb-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-sharding-after-scale-in.components.yaml"
}

create_mongodb_heterogeneous_shards() {
  if kubectl -n "${TEST_NAMESPACE}" get cluster mongodb-hetero-shards >/dev/null 2>&1; then
    log "cluster mongodb-hetero-shards already exists; skipping create"
  else
    create_manifest mongodb-hetero-shards.yaml
  fi

  wait_cluster_running mongodb-hetero-shards
  kubectl get components -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=mongodb-hetero-shards -o wide
  kubectl get components -n "${TEST_NAMESPACE}" -l apps.kubeblocks.io/shard-template=hot -o yaml \
    >"${WORK_DIR}/mongodb-hetero-hot-components.yaml" || true
}

run_mongodb_heterogeneous_offline() {
  create_mongodb_heterogeneous_shards

  local victim
  victim="$(
    kubectl get components -n "${TEST_NAMESPACE}" \
      -l app.kubernetes.io/instance=mongodb-hetero-shards,apps.kubeblocks.io/sharding-name=shard \
      -o json \
      | jq -r '[.items[]
          | select(.metadata.labels["apps.kubeblocks.io/shard-template"] != "hot")
          | .metadata.name][0] // ""'
  )"
  [[ -n "${victim}" ]] || fail "no base-template shard found in mongodb-hetero-shards"

  kubectl get pod -n "${TEST_NAMESPACE}" -o json \
    | jq --arg cluster mongodb-hetero-shards '
        [.items[]
         | select(.metadata.labels["app.kubernetes.io/instance"] == $cluster)
         | {name: .metadata.name, uid: .metadata.uid, component: .metadata.labels["apps.kubeblocks.io/component-name"]}]
      ' >"${WORK_DIR}/mongodb-hetero-before-offline-pods.json"

  patch_cluster_json mongodb-hetero-shards mongodb-hetero-offline-patch.yaml MONGO_OFFLINE_SHARD "${victim}"
  wait_component_absent "${victim}"
  wait_cluster_running mongodb-hetero-shards
  wait_shard_component_count mongodb-hetero-shards shard 2

  if kubectl get component "${victim}" -n "${TEST_NAMESPACE}" >/dev/null 2>&1; then
    fail "offline shard ${victim} still exists"
  fi
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongodb-hetero-shards,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-hetero-after-offline.components.yaml"
}

first_runtime_port_name() {
  local compdef="$1"
  kubectl get componentdefinition "${compdef}" -o json \
    | jq -r '[.spec.runtime.containers[]?.ports[]?.name // empty] | .[0] // empty'
}

cluster_pod_json_by_instance() {
  local cluster="$1"
  local out="$2"
  kubectl get pod -n "${TEST_NAMESPACE}" \
    -l "app.kubernetes.io/instance=${cluster}" \
    -o json >"${out}"
}

wait_shard_component_count() {
  local cluster="$1"
  local sharding="$2"
  local expected="$3"

  log "waiting for ${cluster}/${sharding} shard component count=${expected}"
  for _ in $(seq 1 120); do
    local count
    count="$(
      kubectl get components -n "${TEST_NAMESPACE}" \
        -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=${sharding}" \
        -o json \
        | jq '.items | length'
    )"
    if [[ "${count}" == "${expected}" ]]; then
      return
    fi
    sleep 5
  done

  kubectl get components -n "${TEST_NAMESPACE}" \
    -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=${sharding}" \
    -o wide
  fail "expected ${expected} shard components for ${cluster}/${sharding}"
}

wait_component_absent() {
  local name="$1"

  log "waiting for Component ${name} to be removed"
  for _ in $(seq 1 120); do
    if ! kubectl get component "${name}" -n "${TEST_NAMESPACE}" >/dev/null 2>&1; then
      return
    fi
    sleep 5
  done

  fail "Component ${name} still exists"
}

create_network_dns_cluster() {
  wait_component_definition "${MYSQL_COMPDEF}"
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-dns >/dev/null 2>&1; then
    log "cluster network-dns already exists; skipping create"
  else
    local dns_ip
    dns_ip="$(kubectl -n kube-system get svc kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || true)"
    dns_ip="${dns_ip:-10.96.0.10}"

    create_manifest network-dns.yaml DNS_IP "${dns_ip}"
  fi

  wait_cluster_running network-dns
  cluster_pod_json_by_instance network-dns "${WORK_DIR}/network-dns-pods.json"
  jq -e '
    .items | length == 1
    and .[0].spec.hostNetwork != true
    and .[0].spec.dnsPolicy == "None"
    and (.[0].spec.hostAliases[]? | select(.ip == "10.10.0.12" and (.hostnames | index("legacy-db.internal"))))
    and (.[0].spec.dnsConfig.searches | index("'"${TEST_NAMESPACE}"'.svc.cluster.local"))
  ' "${WORK_DIR}/network-dns-pods.json" >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-dns -o json \
    | jq '.items[] | {name: .metadata.name, hostAliases: .spec.hostAliases, dnsPolicy: .spec.dnsPolicy, dnsConfig: .spec.dnsConfig}' \
    >"${WORK_DIR}/network-dns.result.json"
}

create_network_runtime_hostport_cluster() {
  wait_component_definition "${MYSQL_COMPDEF}"
  local port_name
  port_name="$(first_runtime_port_name "${MYSQL_COMPDEF}")"
  [[ -n "${port_name}" ]] || fail "ComponentDefinition ${MYSQL_COMPDEF} has no runtime container port name"

  if kubectl -n "${TEST_NAMESPACE}" get cluster network-hostport >/dev/null 2>&1; then
    log "cluster network-hostport already exists; skipping create"
  else
    create_manifest network-hostport.yaml PORT_NAME "${port_name}"
  fi

  wait_cluster_running network-hostport
  cluster_pod_json_by_instance network-hostport "${WORK_DIR}/network-hostport-pods.json"
  jq -e --arg port_name "${port_name}" '
    .items | length == 1
    and .[0].spec.hostNetwork != true
    and ([
      .[0].spec.containers[]?.ports[]?
      | select(.name == $port_name and .hostPort == 31001)
    ] | length == 1)
    and ([
      .[0].spec.containers[]?.ports[]?
      | select(.name == "kb11-unknown-port" or .hostPort == 31002)
    ] | length == 0)
  ' "${WORK_DIR}/network-hostport-pods.json" >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-hostport -o json \
    | jq '.items[] | {name: .metadata.name, hostNetwork: .spec.hostNetwork, ports: [.spec.containers[]?.ports[]?]}' \
    >"${WORK_DIR}/network-hostport.result.json"
}

create_mongodb_hostnetwork_cluster() {
  wait_component_definition $MONGO_COMPDEF
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-mongodb-hostnet >/dev/null 2>&1; then
    log "cluster network-mongodb-hostnet already exists; skipping create"
  else
    create_manifest network-mongodb-hostnet.yaml
  fi

  wait_cluster_running network-mongodb-hostnet
  cluster_pod_json_by_instance network-mongodb-hostnet "${WORK_DIR}/network-mongodb-hostnet-pods.json"
  jq -e '
    .items | length == 1
    and .[0].spec.hostNetwork == true
    and (.[0].spec.dnsPolicy == "ClusterFirstWithHostNet" or .[0].spec.dnsPolicy == "Default")
  ' "${WORK_DIR}/network-mongodb-hostnet-pods.json" >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-mongodb-hostnet -o json \
    | jq '.items[] | {name: .metadata.name, node: .spec.nodeName, hostIP: .status.hostIP, podIP: .status.podIP, hostNetwork: .spec.hostNetwork, dnsPolicy: .spec.dnsPolicy, ports: [.spec.containers[]?.ports[]?]}' \
    >"${WORK_DIR}/network-mongodb-hostnet.result.json"
  kubectl -n "${KB_NAMESPACE}" get configmap -o yaml >"${WORK_DIR}/network-hostport-configmaps.yaml"
}

create_redis_sharding_cluster() {
  wait_component_definition "${REDIS_COMPDEF}"
  if kubectl -n "${TEST_NAMESPACE}" get cluster redis-sharding >/dev/null 2>&1; then
    log "cluster redis-sharding already exists; skipping create"
  else
    create_manifest redis-sharding.yaml
  fi
  wait_cluster_running redis-sharding
}

run_redis_sharding_rollout() {
  wait_cluster_running redis-sharding
  delete_rollout_if_exists redis-sharding-rollout

  create_manifest redis-sharding-rollout.yaml
  wait_rollout_succeed redis-sharding-rollout
  wait_cluster_running redis-sharding
  kubectl get rollout redis-sharding-rollout -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/redis-sharding-rollout.result.yaml"
  kubectl get cluster redis-sharding -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/redis-sharding.after-rollout.yaml"
  delete_rollout_if_exists redis-sharding-rollout
}

run_case_upgrade() {
  create_or_select_kind
  install_source_kubeblocks
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_base_cluster
  upgrade_to_target_kubeblocks
}

run_case_network() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_network_dns_cluster
  create_network_runtime_hostport_cluster
  create_mongodb_hostnetwork_cluster
}

run_case_rollout_api() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_test_cluster
  run_mysql_inplace_rollout
  run_mysql_replace_rollout
  run_mysql_create_rollout

  create_redis_sharding_cluster
  run_redis_sharding_rollout
}

run_case_dynamic_instance_template() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_test_cluster
  if ! kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MYSQL_VERSION_CANARY}" \
      '.spec.componentSpecs[] | select(.name == "mysql") | .instances[]? | select(.serviceVersion == $version)' >/dev/null; then
    run_mysql_create_rollout
  fi
  run_dynamic_hot_instance_test
}

run_case_sharding() {
  ensure_namespace "${TEST_NAMESPACE}"
  run_mongodb_sharding_lifecycle
  run_mongodb_heterogeneous_offline
}

cleanup() {
  if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
    kubectl delete namespace "${TEST_NAMESPACE}" --wait=true
  fi
}

run_stage() {
  case "$1" in
    setup-kind) preflight; create_or_select_kind ;;
    setup-kb) preflight; install_target_kubeblocks ;;
    case-upgrade) preflight; run_case_upgrade ;;
    case-network) preflight; run_case_network ;;
    case-rollout-api) preflight; run_case_rollout_api ;;
    case-dynamic-instance-template) preflight; run_case_dynamic_instance_template ;;
    case-sharding) preflight; run_case_sharding ;;
    cleanup) preflight; cleanup ;;
    help|-h|--help) usage ;;
    *) usage; fail "unknown stage: $1" ;;
  esac
}

if [[ "$#" -eq 0 ]]; then
  usage
  exit 0
fi

for stage in "$@"; do
  run_stage "${stage}"
done
