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
WORK_DIR="${WORK_DIR:-/tmp/e2e}"
CACHE_DIR="${CACHE_DIR:-${WORK_DIR}/cache}"
SNAPSHOTTER_VERSION="${SNAPSHOTTER_VERSION:-v8.2.0}"

KB_NAMESPACE="${KB_NAMESPACE:-kb-system}"
TEST_NAMESPACE="${TEST_NAMESPACE:-kb-11-e2e}"
KIND_CLUSTER="${KIND_CLUSTER:-kb-upgrade-103b9}"
SOURCE_VERSION="${SOURCE_VERSION:-1.0.3-beta.9}"
TARGET_VERSION="${TARGET_VERSION:-1.1.0-beta.6}"
AUTO_INSTALLED_ADDONS="${AUTO_INSTALLED_ADDONS:-[\"mysql\",\"mongodb\"]}"

CREATE_KIND="${CREATE_KIND:-true}"
RESET_KIND="${RESET_KIND:-false}"
SKIP_HELM_REPO_UPDATE="${SKIP_HELM_REPO_UPDATE:-true}"

MYSQL_CLUSTER="${MYSQL_CLUSTER:-mysql-cluster}"
MYSQL_COMPDEF="${MYSQL_COMPDEF:-mysql-8.0}"
MYSQL_VERSION_BASE="${MYSQL_VERSION_BASE:-8.0.35}"
MYSQL_VERSION_INPLACE="${MYSQL_VERSION_INPLACE:-8.0.36}"
MYSQL_VERSION_REPLACE="${MYSQL_VERSION_REPLACE:-8.0.37}"
MYSQL_VERSION_CANARY="${MYSQL_VERSION_CANARY:-8.0.38}"
ROLLOUT_INSTANCE_INTERVAL_SECONDS="${ROLLOUT_INSTANCE_INTERVAL_SECONDS:-30}"
ROLLOUT_SCALE_DOWN_DELAY_SECONDS="${ROLLOUT_SCALE_DOWN_DELAY_SECONDS:-30}"
ROLLOUT_CREATE_PROMOTION_DELAY_SECONDS="${ROLLOUT_CREATE_PROMOTION_DELAY_SECONDS:-5}"
ROLLOUT_CREATE_SCALE_DOWN_DELAY_SECONDS="${ROLLOUT_CREATE_SCALE_DOWN_DELAY_SECONDS:-5}"

MONGO_COMPDEF="${MONGO_COMPDEF:-mongodb-1.1.0-alpha.0}"
MONGO_COMPDEF_PREFIX="${MONGO_COMPDEF_PREFIX:-mongodb-}"
MONGO_VERSION="${MONGO_VERSION:-6.0.27}"
MONGO_SHARD_VERSION_BASE="${MONGO_SHARD_VERSION_BASE:-6.0.20}"
MONGO_SHARD_VERSION_TARGET="${MONGO_SHARD_VERSION_TARGET:-6.0.27}"
MONGO_SHARD_COMPDEF="${MONGO_SHARD_COMPDEF:-mongo}"
MONGO_SHARD_COMPDEF_PREFIX="${MONGO_SHARD_COMPDEF_PREFIX:-mongo-}"
MONGO_SHARD_STORAGE="${MONGO_SHARD_STORAGE:-10Gi}"

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
  setup-cache                   Download release CRDs and Helm charts to CACHE_DIR.
  setup-kb                      Install or upgrade KB TARGET_VERSION for non-upgrade cases.
  cleanup                       Delete test namespace.

Test case stages:
  case-upgrade                    Upgrade KB 1.0.3 to KB 1.1 and check MySQL pod/PVC stability.
  case-network                    ComponentNetwork DNS, hostPorts, and hostNetwork.
  case-rollout-api                Rollout API for MySQL and sharding targets.
  case-dynamic-instance-template  Dynamic InstanceSet template adoption.
  case-sharding                   MongoDB sharding scale, offline shard, and heterogeneous shards.

Useful env vars:
  SOURCE_VERSION=1.0.3-beta.9
  TARGET_VERSION=1.1.0-beta.6
  KIND_CLUSTER=kb-upgrade-103b9
  CREATE_KIND=true|false
  RESET_KIND=true|false
  SKIP_HELM_REPO_UPDATE=true|false
  AUTO_INSTALLED_ADDONS='["mysql","mongodb"]'
  MONGO_COMPDEF_PREFIX=mongodb-
  MONGO_SHARD_VERSION_BASE=6.0.20
  MONGO_SHARD_VERSION_TARGET=6.0.27
  MONGO_SHARD_COMPDEF=mongo
  MONGO_SHARD_COMPDEF_PREFIX=mongo-
  ROLLOUT_INSTANCE_INTERVAL_SECONDS=30
  ROLLOUT_SCALE_DOWN_DELAY_SECONDS=30
  ROLLOUT_CREATE_PROMOTION_DELAY_SECONDS=5
  ROLLOUT_CREATE_SCALE_DOWN_DELAY_SECONDS=5
  MANIFEST_DIR=docs/release_notes/v1.1.0/e2e
  WORK_DIR=./docs/release_notes/v1.1.0/e2e/
  CACHE_DIR=./docs/release_notes/v1.1.0/e2e/cache

Examples:
  bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-cache
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh setup-kb
  RESET_KIND=true bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-upgrade
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-network
  CREATE_KIND=false bash docs/release_notes/v1.1.0/e2e/kb-1.1-e2e-reproduce.sh case-rollout-api case-dynamic-instance-template
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

is_supported_mongodb_shard_version() {
  case "$1" in
    6.0.20|6.0.21|6.0.22|6.0.27) return 0 ;;
    *) return 1 ;;
  esac
}

validate_mongodb_shard_versions() {
  is_supported_mongodb_shard_version "${MONGO_SHARD_VERSION_BASE}" \
    || fail "unsupported MONGO_SHARD_VERSION_BASE=${MONGO_SHARD_VERSION_BASE}; use one of 6.0.20, 6.0.21, 6.0.22, 6.0.27"
  is_supported_mongodb_shard_version "${MONGO_SHARD_VERSION_TARGET}" \
    || fail "unsupported MONGO_SHARD_VERSION_TARGET=${MONGO_SHARD_VERSION_TARGET}; use one of 6.0.20, 6.0.21, 6.0.22, 6.0.27"

  [[ "${MONGO_SHARD_VERSION_BASE%.*}" == "${MONGO_SHARD_VERSION_TARGET%.*}" ]] \
    || fail "MongoDB sharding rollout must stay within one minor version: base=${MONGO_SHARD_VERSION_BASE}, target=${MONGO_SHARD_VERSION_TARGET}"
}

prepare_dirs() {
  mkdir -p "${WORK_DIR}" "${CACHE_DIR}"
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
    "ROLLOUT_CREATE_PROMOTION_DELAY_SECONDS=${ROLLOUT_CREATE_PROMOTION_DELAY_SECONDS}"
    "ROLLOUT_CREATE_SCALE_DOWN_DELAY_SECONDS=${ROLLOUT_CREATE_SCALE_DOWN_DELAY_SECONDS}"
    "MONGO_COMPDEF=${MONGO_COMPDEF}"
    "MONGO_COMPDEF_PREFIX=${MONGO_COMPDEF_PREFIX}"
    "MONGO_VERSION=${MONGO_VERSION}"
    "MONGO_SHARD_VERSION_BASE=${MONGO_SHARD_VERSION_BASE}"
    "MONGO_SHARD_VERSION_TARGET=${MONGO_SHARD_VERSION_TARGET}"
    "MONGO_SHARD_COMPDEF=${MONGO_SHARD_COMPDEF}"
    "MONGO_SHARD_COMPDEF_PREFIX=${MONGO_SHARD_COMPDEF_PREFIX}"
    "MONGO_SHARD_STORAGE=${MONGO_SHARD_STORAGE}"
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
    log "creating single-node kind cluster ${KIND_CLUSTER} without proxy env"
    noproxy
    kind create cluster --name "${KIND_CLUSTER}" --config "$(manifest_path kind-single-node.yaml)"
    proxy
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

resolve_component_definition() {
  local prefix="$1"
  kubectl get componentdefinitions -o json \
    | jq -r --arg prefix "${prefix}" '
        [.items[].metadata.name | select(startswith($prefix))]
        | sort
        | .[0] // ""
      '
}

wait_component_definition_prefix() {
  local prefix="$1"
  printf '\n[%s] waiting for ComponentDefinition prefix %s\n' "$(date '+%H:%M:%S')" "${prefix}" >&2
  for _ in $(seq 1 120); do
    local name
    name="$(resolve_component_definition "${prefix}")"
    if [[ -n "${name}" ]]; then
      printf '%s\n' "${name}"
      return
    fi
    sleep 5
  done
  fail "ComponentDefinition with prefix ${prefix} did not appear"
}

ensure_helm_repo() {
  require_cmd helm
  if ! helm repo list -o json | jq -e '.[] | select(.name == "kubeblocks")' >/dev/null; then
    helm repo add kubeblocks https://apecloud.github.io/helm-charts
  fi
  if [[ "${SKIP_HELM_REPO_UPDATE}" != "true" ]]; then
    helm repo update kubeblocks
  else
    log "skipping helm repo update because SKIP_HELM_REPO_UPDATE=true"
  fi
}

kubeblocks_crds_cache_path() {
  local version="$1"
  printf '%s/kubeblocks_crds-v%s.yaml' "${CACHE_DIR}" "${version}"
}

kubeblocks_chart_cache_path() {
  local version="$1"
  printf '%s/kubeblocks-%s.tgz' "${CACHE_DIR}" "${version}"
}

snapshot_crds_cache_dir() {
  printf '%s/snapshot-crds-%s' "${CACHE_DIR}" "${SNAPSHOTTER_VERSION}"
}

download_file_if_missing() {
  local url="$1"
  local path="$2"
  local tmp
  if [[ -s "${path}" ]]; then
    log "using cached file ${path}"
    return
  fi

  require_cmd curl
  tmp="${path}.tmp"
  log "downloading ${url} to ${path}"
  wget --tries=5 --waitretry=3 --timeout=180 --connect-timeout=20 -O "${tmp}" "${url}"
  mv "${tmp}" "${path}"
}

cache_snapshot_crds() {
  local dir base url name
  dir="$(snapshot_crds_cache_dir)"
  mkdir -p "${dir}"
  base="https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/${SNAPSHOTTER_VERSION}/client/config/crd"
  for name in \
    snapshot.storage.k8s.io_volumesnapshotclasses.yaml \
    snapshot.storage.k8s.io_volumesnapshots.yaml \
    snapshot.storage.k8s.io_volumesnapshotcontents.yaml; do
    url="${base}/${name}"
    download_file_if_missing "${url}" "${dir}/${name}"
  done
}

cache_kubeblocks_crds() {
  local version="$1"
  local path url
  path="$(kubeblocks_crds_cache_path "${version}")"
  url="https://github.com/apecloud/kubeblocks/releases/download/v${version}/kubeblocks_crds.yaml"
  download_file_if_missing "${url}" "${path}"
}

cache_kubeblocks_chart() {
  local version="$1"
  local path
  path="$(kubeblocks_chart_cache_path "${version}")"
  if [[ -s "${path}" ]]; then
    log "using cached KubeBlocks chart ${path}"
    return
  fi

  ensure_helm_repo
  log "downloading kubeblocks chart ${version} to ${CACHE_DIR}"
  helm pull kubeblocks/kubeblocks --version "${version}" --destination "${CACHE_DIR}"
  [[ -s "${path}" ]] || fail "expected cached chart not found: ${path}"
}

prepare_release_cache() {
  cache_snapshot_crds
  cache_kubeblocks_crds "${SOURCE_VERSION}"
  cache_kubeblocks_crds "${TARGET_VERSION}"
  cache_kubeblocks_chart "${SOURCE_VERSION}"
  cache_kubeblocks_chart "${TARGET_VERSION}"
}

install_snapshot_crds() {
  log "installing Snapshot CRDs before Helm"
  if kubectl get crd volumesnapshots.snapshot.storage.k8s.io >/dev/null 2>&1; then
    log "Snapshot CRDs already exist"
  else
    cache_snapshot_crds
    kubectl create -f "$(snapshot_crds_cache_dir)"
  fi
}

install_kubeblocks() {
  local kb_version="$1"
  ensure_namespace "${KB_NAMESPACE}"
  install_snapshot_crds
  cache_kubeblocks_crds "${kb_version}"
  cache_kubeblocks_chart "${kb_version}"

  log "creating or replacing KubeBlocks CRDs for v${kb_version}"
  kubectl create -f "$(kubeblocks_crds_cache_path "${kb_version}")" || kubectl replace -f "$(kubeblocks_crds_cache_path "${kb_version}")"

  if helm -n "${KB_NAMESPACE}" status kubeblocks >/dev/null 2>&1; then
    log "upgrading kubeblocks chart to ${kb_version}"
    helm -n "${KB_NAMESPACE}" upgrade kubeblocks "$(kubeblocks_chart_cache_path "${kb_version}")" \
      --skip-crds \
      --reuse-values \
      --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"
  else
    log "installing kubeblocks chart ${kb_version}"
    helm install kubeblocks "$(kubeblocks_chart_cache_path "${kb_version}")" \
      -n "${KB_NAMESPACE}" \
      --create-namespace \
      --skip-crds \
      --set-json "autoInstalledAddons=${AUTO_INSTALLED_ADDONS}"
  fi

  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks-dataprotection
  wait_component_definition_prefix "${MYSQL_COMPDEF}" >/dev/null
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

assert_mysql_stable_pods_unchanged() {
  local before="$1"
  local after="$2"

  log "checking create rollout keeps existing stable MySQL pods unchanged"
  diff -u \
    <(jq '[.[] | select(.template == "") | {name, uid}] | sort_by(.name)' "${before}") \
    <(jq '[.[] | select(.template == "") | {name, uid}] | sort_by(.name)' "${after}")
}

assert_mysql_create_canary_present() {
  local pod_snapshot="$1"

  log "checking create rollout produced a MySQL ${MYSQL_VERSION_CANARY} instance template pod"
  jq -e --arg version "${MYSQL_VERSION_CANARY}" '
    any(.[]; .template != "" and any(.images[]; .name == "mysql" and (.image | endswith(":" + $version))))
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
  ensure_namespace "${TEST_NAMESPACE}"

  if kubectl -n "${TEST_NAMESPACE}" get cluster "${MYSQL_CLUSTER}" >/dev/null 2>&1; then
    log "cluster ${MYSQL_CLUSTER} already exists; skipping create"
  else
    create_manifest mysql-rollout-base.yaml
  fi

  wait_cluster_running "${MYSQL_CLUSTER}"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" \
    -o jsonpath='{.spec.componentSpecs[0].componentDef}{"\t"}{.spec.componentSpecs[0].serviceVersion}{"\n"}'
  assert_mysql_primary_standby_cluster "${MYSQL_CLUSTER}"
}

assert_mysql_primary_standby_cluster() {
  local cluster="$1"
  log "checking MySQL primary-standby example ${cluster}"
  kubectl get cluster "${cluster}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg compdef "${MYSQL_COMPDEF}" --arg version "${MYSQL_VERSION_BASE}" '
        .spec.componentSpecs[]
        | select(.name == "mysql")
        | (.componentDef | startswith($compdef))
          and .serviceVersion == $version
          and .replicas == 2
      ' >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l "app.kubernetes.io/instance=${cluster}" -o json \
    | jq -e '.items | length == 2 and all(.status.phase == "Running")' >/dev/null
}

assert_mysql_standalone_cluster() {
  local cluster="$1"
  log "checking MySQL standalone example ${cluster}"
  kubectl get cluster "${cluster}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg compdef "${MYSQL_COMPDEF}" --arg version "${MYSQL_VERSION_BASE}" '
        .spec.componentSpecs[]
        | select(.name == "mysql")
        | (.componentDef | startswith($compdef))
          and .serviceVersion == $version
          and .replicas == 1
      ' >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l "app.kubernetes.io/instance=${cluster}" -o json \
    | jq -e '.items | length == 1 and .[0].status.phase == "Running"' >/dev/null
}

assert_mongodb_standalone_cluster() {
  local cluster="$1"
  log "checking MongoDB standalone host-network example ${cluster}"
  kubectl get cluster "${cluster}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MONGO_VERSION}" '
        .spec.componentSpecs[]
        | select(.name == "mongodb")
        | .componentDef == "mongodb-1.0"
          and .serviceVersion == $version
          and .replicas == 1
      ' >/dev/null
  kubectl get pod -n "${TEST_NAMESPACE}" -l "app.kubernetes.io/instance=${cluster}" -o json \
    | jq -e '.items | length == 1 and .[0].status.phase == "Running"' >/dev/null
}

create_mysql_base_cluster() {
  create_mysql_test_cluster
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-before-upgrade-pods.json"
  snapshot_all_managed_pods "${WORK_DIR}/before-upgrade-managed-pods.json"
  snapshot_all_pvcs "${WORK_DIR}/before-upgrade-pvcs.json"
}

upgrade_to_target_kubeblocks() {
  install_kubeblocks "${TARGET_VERSION}"
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks
  wait_deploy_if_exists "${KB_NAMESPACE}" kubeblocks-dataprotection
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
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-before-create-rollout-pods.json"

  create_manifest mysql-create-8038.yaml
  wait_rollout_succeed mysql-create-8038
  wait_cluster_running "${MYSQL_CLUSTER}"
  kubectl get rollout mysql-create-8038 -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-create-8038.result.yaml"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o yaml >"${WORK_DIR}/mysql-after-create-rollout.cluster.yaml"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-after-create-rollout-pods.json"
  assert_mysql_create_canary_present "${WORK_DIR}/mysql-after-create-rollout-pods.json"
  assert_mysql_stable_pods_unchanged \
    "${WORK_DIR}/mysql-before-create-rollout-pods.json" \
    "${WORK_DIR}/mysql-after-create-rollout-pods.json"

  # delete_rollout_if_exists mysql-create-8038
}

run_dynamic_mysql_instance_template_test() {
  wait_cluster_running "${MYSQL_CLUSTER}"
  local dynamic_template="kb11-dynamic-adopt"
  local default_pod default_ordinal
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-dynamic-before.json"
  default_pod="$(
    jq -r '[.[] | select(.template == "")] | sort_by(.name) | .[0].name // ""' \
      "${WORK_DIR}/mysql-dynamic-before.json"
  )"
  [[ -n "${default_pod}" ]] || fail "no MySQL pod from default template found"
  default_ordinal="${default_pod##*-}"
  [[ "${default_ordinal}" =~ ^[0-9]+$ ]] || fail "cannot parse ordinal from pod ${default_pod}"

  log "adopting MySQL pod ${default_pod} from default template into ${dynamic_template}"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
    | jq --arg name "${dynamic_template}" --argjson ordinal "${default_ordinal}" '
        (.spec.componentSpecs[] | select(.name == "mysql")) |= (
          .flatInstanceOrdinal = true
          | .componentDef as $componentDef
          | .serviceVersion as $serviceVersion
          | .instances = ((.instances // []) | map(select(.name != $name)) + [{
              "name": $name,
              "compDef": $componentDef,
              "serviceVersion": $serviceVersion,
              "replicas": 1,
              "ordinals": {"discrete": [$ordinal]},
              "labels": {"workload-tier": "dynamic-adopt"}
            }])
        )' \
    | kubectl replace -f -

  wait_cluster_running "${MYSQL_CLUSTER}"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-dynamic-after-adopt.json"
  jq -e --arg pod "${default_pod}" --arg template "${dynamic_template}" '
      .[] | select(.name == $pod and .template == $template)
    ' "${WORK_DIR}/mysql-dynamic-after-adopt.json" >/dev/null
  kubectl get pod "${default_pod}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg template "${dynamic_template}" '
        {
          name: .metadata.name,
          uid: .metadata.uid,
          labels: .metadata.labels,
          template: (.metadata.labels["apps.kubeblocks.io/instance-template"] // .metadata.labels["workloads.kubeblocks.io/template-name"] // "")
        }
        | select(.template == $template and .labels["workload-tier"] == "dynamic-adopt")
      ' >"${WORK_DIR}/mysql-dynamic-adopted-pod.json"

  log "giving MySQL pod ${default_pod} back to default template"
  kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
    | jq --arg name "${dynamic_template}" '
        (.spec.componentSpecs[] | select(.name == "mysql")) |= (
          .instances = ((.instances // []) | map(select(.name != $name)))
        )' \
    | kubectl replace -f -

  wait_cluster_running "${MYSQL_CLUSTER}"
  snapshot_cluster_pods "${MYSQL_CLUSTER}" "${WORK_DIR}/mysql-dynamic-after-giveback.json"
  jq -e --arg pod "${default_pod}" '
      .[] | select(.name == $pod and .template == "")
    ' "${WORK_DIR}/mysql-dynamic-after-giveback.json" >/dev/null
}

create_mongodb_sharding_cluster() {
  validate_mongodb_shard_versions
  wait_component_definition_prefix "${MONGO_SHARD_COMPDEF_PREFIX}" >/dev/null
  if kubectl -n "${TEST_NAMESPACE}" get cluster mongo-sharding >/dev/null 2>&1; then
    log "cluster mongo-sharding already exists; skipping create"
  else
    create_manifest mongodb-sharding.yaml
  fi

  wait_cluster_running mongo-sharding
  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MONGO_SHARD_VERSION_BASE}" '
        ([.spec.componentSpecs[]?.replicas, .spec.shardings[]?.template.replicas] | all(. != null and . > 0))
        and
        ([.spec.componentSpecs[]?.serviceVersion, .spec.shardings[]?.template.serviceVersion] | all(. == $version))
      ' >/dev/null
  kubectl get components -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=mongo-sharding -o wide
}

assert_mongodb_sharding_shards() {
  local cluster="$1"
  local expected="$2"
  kubectl get cluster "${cluster}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --argjson expected "${expected}" '
        .spec.shardings[]
        | select(.name == "shard")
        | .shards == $expected
      ' >/dev/null
}

mongo_sharding_exec() {
  local cluster="$1"
  local script="$2"
  local user password
  user="$(kubectl -n "${TEST_NAMESPACE}" get secret "${cluster}-config-server-account-root" -o go-template='{{.data.username | base64decode}}')"
  password="$(kubectl -n "${TEST_NAMESPACE}" get secret "${cluster}-config-server-account-root" -o go-template='{{.data.password | base64decode}}')"
  kubectl -n "${TEST_NAMESPACE}" exec "${cluster}-mongos-0" -c mongos -- \
    mongosh --quiet --username "${user}" --password "${password}" --authenticationDatabase admin --eval "${script}"
}

seed_mongodb_sharding_data() {
  local cluster="$1"
  local out="${WORK_DIR}/${cluster}-sharding-data-seed.json"
  local live_shards
  local js
  live_shards="$(
    kubectl get components -n "${TEST_NAMESPACE}" \
      -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=shard" \
      -o json \
      | jq -c '[.items[] | select(.status.phase == "Running") | .metadata.name] | sort'
  )"
  [[ "${live_shards}" != "[]" ]] || fail "no running MongoDB shard components found for ${cluster}"

  log "seeding MongoDB data through ${cluster} mongos, one marker database per shard"
  js="$(cat <<'JS'
const clusterName = '__CLUSTER__';
const markerId = 'kb11-sharding-marker';
const metaDbName = 'kb11_e2e_meta';
const liveShards = __LIVE_SHARDS__;
const sampleCount = 20;
const listedShards = db.adminCommand({listShards: 1}).shards.map(s => s._id).sort();
if (liveShards.length === 0) {
  throw new Error('no MongoDB shards found');
}
const metaEnabled = db.adminCommand({enableSharding: metaDbName, primaryShard: liveShards[0]});
if (!metaEnabled.ok) {
  throw new Error('enableSharding failed for ' + metaDbName + ': ' + tojson(metaEnabled));
}
const metaDb = db.getSiblingDB(metaDbName);
const seeded = [];
for (let i = 0; i < liveShards.length; i++) {
  const shard = liveShards[i];
  if (!listedShards.includes(shard)) {
    throw new Error('live shard component ' + shard + ' is not listed by mongos');
  }
  const dbName = 'kb11_e2e_marker_' + i;
  const enabled = db.adminCommand({enableSharding: dbName, primaryShard: shard});
  if (!enabled.ok) {
    throw new Error('enableSharding failed for ' + dbName + ' on ' + shard + ': ' + tojson(enabled));
  }
  const markerDb = db.getSiblingDB(dbName);
  markerDb.marker.updateOne(
    {_id: markerId},
    {$set: {cluster: clusterName, dbName, markerIndex: i, seededAt: new Date()}},
    {upsert: true}
  );
  const doc = markerDb.marker.findOne({_id: markerId});
  if (!doc || doc.dbName !== dbName || doc.markerIndex !== i || doc.cluster !== clusterName) {
    throw new Error('marker verification failed for ' + dbName);
  }
  markerDb.integrity.deleteMany({markerId});
  const docs = [];
  for (let n = 0; n < sampleCount; n++) {
    docs.push({
      _id: markerId + '-' + i + '-' + n,
      markerId,
      cluster: clusterName,
      dbName,
      markerIndex: i,
      sampleIndex: n,
      checksum: i * 100000 + n,
      payload: 'kb11-sharding-integrity-' + i + '-' + n
    });
  }
  markerDb.integrity.insertMany(docs);
  const writtenDocs = markerDb.integrity.find({markerId}).toArray();
  const checksum = writtenDocs.reduce((sum, item) => sum + item.checksum, 0);
  const malformed = writtenDocs.some((item) =>
    item.cluster !== clusterName ||
    item.dbName !== dbName ||
    item.markerIndex !== i ||
    item.markerId !== markerId
  );
  if (writtenDocs.length !== sampleCount || checksum !== docs.reduce((sum, item) => sum + item.checksum, 0) || malformed) {
    throw new Error('integrity sample verification failed for ' + dbName);
  }
  seeded.push({dbName, markerIndex: i, initialShard: shard, sampleCount, checksum});
}
metaDb.shardingMarkers.updateOne(
  {_id: clusterName},
  {$set: {cluster: clusterName, markerId, seeded, expectedCount: seeded.length, updatedAt: new Date()}},
  {upsert: true}
);
print(JSON.stringify({ok: 1, cluster: clusterName, seeded}, null, 2));
JS
)"
  js="${js//__CLUSTER__/${cluster}}"
  js="${js//__LIVE_SHARDS__/${live_shards}}"
  mongo_sharding_exec "${cluster}" "${js}" >"${out}"
}

verify_mongodb_sharding_data() {
  local cluster="$1"
  local phase="$2"
  local out="${WORK_DIR}/${cluster}-sharding-data-${phase}.json"
  local js
  log "verifying MongoDB sharding data for ${cluster} (${phase})"
  js="$(cat <<'JS'
const clusterName = '__CLUSTER__';
const meta = db.getSiblingDB('kb11_e2e_meta').shardingMarkers.findOne({_id: clusterName});
if (!meta || !Array.isArray(meta.seeded) || meta.seeded.length === 0) {
  throw new Error('missing sharding marker metadata for ' + clusterName);
}
const verified = [];
for (const expected of meta.seeded) {
  const markerDb = db.getSiblingDB(expected.dbName);
  const doc = markerDb.marker.findOne({_id: meta.markerId});
  if (!doc) {
    throw new Error('missing marker document in ' + expected.dbName);
  }
  if (doc.dbName !== expected.dbName || doc.markerIndex !== expected.markerIndex || doc.cluster !== clusterName) {
    throw new Error('marker mismatch in ' + expected.dbName + ': ' + JSON.stringify(doc));
  }
  const docs = markerDb.integrity.find({markerId: meta.markerId}).toArray();
  const checksum = docs.reduce((sum, item) => sum + item.checksum, 0);
  const malformed = docs.some((item) =>
    item.cluster !== clusterName ||
    item.dbName !== expected.dbName ||
    item.markerIndex !== expected.markerIndex ||
    item.markerId !== meta.markerId
  );
  if (docs.length !== expected.sampleCount || checksum !== expected.checksum || malformed) {
    throw new Error(
      'integrity sample mismatch in ' + expected.dbName +
      ': expected count=' + expected.sampleCount +
      ', checksum=' + expected.checksum +
      '; got count=' + docs.length +
      ', checksum=' + checksum
    );
  }
  verified.push({dbName: expected.dbName, markerIndex: expected.markerIndex, sampleCount: docs.length, checksum});
}
if (verified.length !== meta.expectedCount) {
  throw new Error('verified marker count mismatch: expected ' + meta.expectedCount + ', got ' + verified.length);
}
print(JSON.stringify({ok: 1, cluster: clusterName, phase: '__PHASE__', verified}, null, 2));
JS
)"
  js="${js//__CLUSTER__/${cluster}}"
  js="${js//__PHASE__/${phase}}"
  mongo_sharding_exec "${cluster}" "${js}" >"${out}"
}

run_mongodb_sharding_scale() {
  create_mongodb_sharding_cluster
  seed_mongodb_sharding_data mongo-sharding

  log "patching mongo-sharding spec.shardings[name=shard].shards to 4"
  patch_cluster_json mongo-sharding mongodb-sharding-scale-out-patch.yaml
  assert_mongodb_sharding_shards mongo-sharding 4
  wait_cluster_running mongo-sharding
  wait_shard_component_count mongo-sharding shard 4
  verify_mongodb_sharding_data mongo-sharding after-scale-out
  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/mongodb-sharding-after-scale-out.cluster.yaml"
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-sharding-after-scale-out.components.yaml"

  run_mongodb_sharding_offline_replacement
  run_mongodb_sharding_offline_scale_in
}

run_mongodb_sharding_offline_replacement() {
  local victim
  victim="$(pick_mongodb_offline_ready_base_shard mongo-sharding offline-replacement)"

  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o json \
    | jq '[.items[] | .metadata.name] | sort' \
    >"${WORK_DIR}/mongodb-sharding-before-offline-replacement.components.json"
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-before-offline-replacement-pods.json"

  log "patching mongo-sharding offline shard ${victim} with unchanged shards=4"
  patch_cluster_json mongo-sharding mongodb-sharding-offline-replacement-patch.yaml MONGO_OFFLINE_SHARD "${victim}"
  assert_mongodb_sharding_shards mongo-sharding 4
  wait_component_absent "${victim}" mongo-sharding
  wait_cluster_running mongo-sharding
  wait_shard_component_count mongo-sharding shard 4
  verify_mongodb_sharding_data mongo-sharding after-offline-replacement

  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/mongodb-sharding-after-offline-replacement.cluster.yaml"
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o json \
    | jq '[.items[] | .metadata.name] | sort' \
    >"${WORK_DIR}/mongodb-sharding-after-offline-replacement.components.json"
  jq -e --slurp --arg victim "${victim}" '
        .[0] as $before
        | .[1] as $after
        | ($before | index($victim)) != null
          and (($after | index($victim)) == null)
          and ($after | length == 4)
          and ((($before - [$victim]) - $after) | length == 0)
          and (($after - $before) | length == 1)
      ' \
      "${WORK_DIR}/mongodb-sharding-before-offline-replacement.components.json" \
      "${WORK_DIR}/mongodb-sharding-after-offline-replacement.components.json" >/dev/null
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-after-offline-replacement-pods.json"
  assert_non_victim_shard_pods_unchanged \
    "${WORK_DIR}/mongodb-sharding-before-offline-replacement-pods.json" \
    "${WORK_DIR}/mongodb-sharding-after-offline-replacement-pods.json" \
    "${victim}"
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-sharding-after-offline-replacement.components.yaml"
}

run_mongodb_sharding_offline_scale_in() {
  local victim
  victim="$(pick_mongodb_offline_ready_base_shard mongo-sharding offline-scale-in)"

  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o json \
    | jq '[.items[] | .metadata.name] | sort' \
    >"${WORK_DIR}/mongodb-sharding-before-offline-scale-in.components.json"
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-before-offline-scale-in-pods.json"

  log "patching mongo-sharding offline shard ${victim} and shards=3"
  patch_cluster_json mongo-sharding mongodb-sharding-offline-scale-in-patch.yaml MONGO_OFFLINE_SHARD "${victim}"
  assert_mongodb_sharding_shards mongo-sharding 3
  wait_component_absent "${victim}" mongo-sharding
  wait_cluster_running mongo-sharding
  wait_shard_component_count mongo-sharding shard 3
  verify_mongodb_sharding_data mongo-sharding after-offline-scale-in

  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/mongodb-sharding-after-offline-scale-in.cluster.yaml"
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o json \
    | jq '[.items[] | .metadata.name] | sort' \
    >"${WORK_DIR}/mongodb-sharding-after-offline-scale-in.components.json"
  jq -e --slurp --arg victim "${victim}" '
        .[0] as $before
        | .[1] as $after
        | ($before | index($victim)) != null
          and (($after | index($victim)) == null)
          and ($after | length == 3)
          and (($after - ($before - [$victim])) | length == 0)
      ' \
      "${WORK_DIR}/mongodb-sharding-before-offline-scale-in.components.json" \
      "${WORK_DIR}/mongodb-sharding-after-offline-scale-in.components.json" >/dev/null
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-after-offline-scale-in-pods.json"
  assert_non_victim_shard_pods_unchanged \
    "${WORK_DIR}/mongodb-sharding-before-offline-scale-in-pods.json" \
    "${WORK_DIR}/mongodb-sharding-after-offline-scale-in-pods.json" \
    "${victim}"
  kubectl get components -n "${TEST_NAMESPACE}" \
    -l app.kubernetes.io/instance=mongo-sharding,apps.kubeblocks.io/sharding-name=shard \
    -o yaml >"${WORK_DIR}/mongodb-sharding-after-offline-scale-in.components.yaml"
}

create_mongodb_heterogeneous_shards() {
  validate_mongodb_shard_versions
  wait_component_definition_prefix "${MONGO_SHARD_COMPDEF_PREFIX}" >/dev/null
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

run_mongodb_heterogeneous_test() {
  create_mongodb_heterogeneous_shards
  seed_mongodb_sharding_data mongodb-hetero-shards
  run_mongodb_heterogeneous_version_canary
  verify_mongodb_sharding_data mongodb-hetero-shards after-version-canary
}

write_mongodb_shard_offline_report() {
  local cluster="$1"
  local out="$2"
  local js
  js="$(cat <<'JS'
const cfg = db.getSiblingDB('config');
const chunks = cfg.chunks;
const collectionByUUID = {};
cfg.collections.find().toArray().forEach((collection) => {
  if (collection.uuid !== undefined) {
    collectionByUUID[String(collection.uuid)] = collection._id;
  }
});
const report = cfg.shards.find().toArray().map((shard) => {
  const shardName = shard._id;
  const dbsToMove = cfg.databases.find({primary: shardName}).toArray().map((database) => database._id).sort();
  const namespaceChunks = chunks.aggregate([
    {$match: {shard: shardName}},
    {$group: {_id: {$ifNull: ["$ns", "$uuid"]}, chunks: {$sum: 1}}},
    {$sort: {chunks: -1}},
    {$limit: 8}
  ]).toArray().map((entry) => {
    const key = String(entry._id);
    return {namespace: collectionByUUID[key] || key, chunks: entry.chunks};
  });
  const chunkCount = chunks.countDocuments({shard: shardName});
  const jumboChunkCount = chunks.countDocuments({shard: shardName, jumbo: true});
  const blockers = [];
  if (shard.draining === true) {
    blockers.push('shard is already draining');
  }
  if (chunkCount > 0) {
    blockers.push(chunkCount + ' chunks still live on this shard');
  }
  if (jumboChunkCount > 0) {
    blockers.push(jumboChunkCount + ' jumbo chunks need manual handling');
  }
  if (dbsToMove.length > 0) {
    blockers.push('primary databases must be moved or dropped: ' + dbsToMove.join(','));
  }
  return {
    shard: shardName,
    draining: shard.draining === true,
    chunks: chunkCount,
    jumboChunks: jumboChunkCount,
    dbsToMove,
    namespaceChunks,
    offlineReady: blockers.length === 0,
    blockers
  };
}).sort((a, b) => a.shard.localeCompare(b.shard));
print(JSON.stringify(report, null, 2));
JS
)"
  mongo_sharding_exec "${cluster}" "${js}" >"${out}"
}

pick_mongodb_offline_ready_base_shard() {
  local cluster="$1"
  local phase="$2"
  local report="${WORK_DIR}/${cluster}-${phase}-offline-readiness.json"
  local base_shards
  local victim

  base_shards="$(
    kubectl get components -n "${TEST_NAMESPACE}" \
      -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=shard" \
      -o json \
      | jq -c '[.items[]
          | select(.metadata.labels["apps.kubeblocks.io/shard-template"] != "hot")
          | .metadata.name] | sort'
  )"
  [[ "${base_shards}" != "[]" ]] || fail "no base-template shard found in ${cluster}"

  write_mongodb_shard_offline_report "${cluster}" "${report}"
  victim="$(
    jq -r --argjson baseShards "${base_shards}" '
      [.[] as $shard
       | select(($baseShards | index($shard.shard))
                and $shard.offlineReady)
       | $shard
       | .shard][0] // ""
    ' "${report}"
  )"
  if [[ -z "${victim}" ]]; then
    jq . "${report}"
    fail "no base-template shard is ready for offline in ${cluster}; inspect ${report}"
  fi
  printf '\n[%s] selected offline-ready shard %s for %s (%s)\n' "$(date '+%H:%M:%S')" "${victim}" "${cluster}" "${phase}" >&2
  printf '%s\n' "${victim}"
}

run_mongodb_heterogeneous_version_canary() {
  validate_mongodb_shard_versions
  if [[ "${MONGO_SHARD_VERSION_TARGET}" == "${MONGO_SHARD_VERSION_BASE}" ]]; then
    log "MONGO_SHARD_VERSION_TARGET equals MONGO_SHARD_VERSION_BASE; skipping MongoDB shard template version canary"
    return
  fi

  kubectl get pod -n "${TEST_NAMESPACE}" -o json \
    | jq --arg cluster mongodb-hetero-shards '
        [.items[]
         | select(.metadata.labels["app.kubernetes.io/instance"] == $cluster)
         | {name: .metadata.name, uid: .metadata.uid, component: .metadata.labels["apps.kubeblocks.io/component-name"], shardTemplate: .metadata.labels["apps.kubeblocks.io/shard-template"]}]
      ' >"${WORK_DIR}/mongodb-hetero-before-version-pods.json"

  patch_cluster_json mongodb-hetero-shards mongodb-hetero-version-patch.yaml
  wait_cluster_running mongodb-hetero-shards

  kubectl get pod -n "${TEST_NAMESPACE}" -o json \
    | jq --arg cluster mongodb-hetero-shards '
        [.items[]
         | select(.metadata.labels["app.kubernetes.io/instance"] == $cluster)
         | {name: .metadata.name, uid: .metadata.uid, component: .metadata.labels["apps.kubeblocks.io/component-name"], shardTemplate: .metadata.labels["apps.kubeblocks.io/shard-template"], images: [.spec.containers[] | {name, image}]}]
      ' >"${WORK_DIR}/mongodb-hetero-after-version-pods.json"
  kubectl get components -n "${TEST_NAMESPACE}" -l apps.kubeblocks.io/shard-template=hot -o yaml \
    >"${WORK_DIR}/mongodb-hetero-after-version-hot.components.yaml"
  kubectl get components -n "${TEST_NAMESPACE}" -l apps.kubeblocks.io/shard-template=hot -o json \
    | jq -e --arg version "${MONGO_SHARD_VERSION_TARGET}" '
        .items | length == 1
        and .[0].spec.serviceVersion == $version
      ' >/dev/null
}

first_runtime_port_name() {
  local compdef="$1"
  local resolved
  resolved="$(wait_component_definition_prefix "${compdef}")"
  kubectl get componentdefinition "${resolved}" -o json \
    | jq -r '[.spec.runtime.containers[]?.ports[]?.name // empty] | .[0] // empty'
}

cluster_pod_json_by_instance() {
  local cluster="$1"
  local out="$2"
  kubectl get pod -n "${TEST_NAMESPACE}" \
    -l "app.kubernetes.io/instance=${cluster}" \
    -o json >"${out}"
}

snapshot_sharding_pods() {
  local cluster="$1"
  local sharding="$2"
  local out="$3"
  kubectl get pod -n "${TEST_NAMESPACE}" -o json \
    | jq --arg cluster "${cluster}" --arg sharding "${sharding}" '
        [.items[]
         | select(.metadata.labels["app.kubernetes.io/instance"] == $cluster)
         | select(.metadata.labels["apps.kubeblocks.io/sharding-name"] == $sharding)
         | {
             name: .metadata.name,
             uid: .metadata.uid,
             component: (.metadata.labels["workloads.kubeblocks.io/instance"] // ($cluster + "-" + .metadata.labels["apps.kubeblocks.io/component-name"])),
             readyTime: ([.status.conditions[]? | select(.type == "Ready" and .status == "True") | .lastTransitionTime][0] // ""),
             serviceVersion: (.metadata.labels["apps.kubeblocks.io/service-version"] // ""),
             images: [.spec.containers[] | {name, image}]
           }]
        | sort_by(.name)
      ' >"${out}"
}

assert_sharding_rollout_status() {
  local rollout="$1"
  local sharding="$2"
  local source_version="$3"
  kubectl get rollout "${rollout}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg sharding "${sharding}" --arg version "${source_version}" '
        .status.state == "Succeed"
        and (.status.shardings | type == "array")
        and ([
          .status.shardings[]
          | select(.name == $sharding)
          | select(.serviceVersion == $version)
          | select(.replicas == .newReplicas and .replicas == .rolledOutReplicas)
        ] | length == 1)
      ' >/dev/null
}

assert_sharding_pods_replaced_with_version() {
  local before="$1"
  local after="$2"
  local expected_version="$3"
  local diff_out="${WORK_DIR}/$(basename "${after}" .json).rollout.diff"

  jq -e --arg version "${expected_version}" '
    length > 0
    and all(.serviceVersion == $version)
  ' "${after}" >/dev/null

  if ! jq -e --slurp '
      .[0] as $before
      | .[1] as $after
      | ($before | length) > 0
        and ($before | length) == ($after | length)
        and (($before | map(.component) | sort) == ($after | map(.component) | sort))
        and all($before[]; . as $old
          | ($after[] | select(.component == $old.component) | .uid) != $old.uid)
    ' "${before}" "${after}" >"${diff_out}"; then
    cat "${diff_out}"
    fail "MongoDB sharding rollout did not replace every shard pod; inspect ${diff_out}"
  fi
}

assert_non_victim_shard_pods_unchanged() {
  local before="$1"
  local after="$2"
  local victim="$3"
  local diff_out="${WORK_DIR}/$(basename "${after}" .json).non-victim.diff"
  local survivors

  survivors="$(
    jq -c --arg victim "${victim}" '
      [.[] | select(.component != $victim) | .component] | unique
    ' "${before}"
  )"

  if ! diff -u \
      <(jq --argjson survivors "${survivors}" '[.[] | select(.component as $component | $survivors | index($component)) | {name, uid, component}] | sort_by(.name)' "${before}") \
      <(jq --argjson survivors "${survivors}" '[.[] | select(.component as $component | $survivors | index($component)) | {name, uid, component}] | sort_by(.name)' "${after}") \
      >"${diff_out}"; then
    cat "${diff_out}"
    fail "non-victim shard pods changed while taking ${victim} offline; inspect ${diff_out}"
  fi
}

wait_shard_component_count() {
  local cluster="$1"
  local sharding="$2"
  local expected="$3"

  log "waiting for ${cluster}/${sharding} shard component count=${expected} and all Running"
  for _ in $(seq 1 120); do
    if kubectl get components -n "${TEST_NAMESPACE}" \
        -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=${sharding}" \
        -o json \
        | jq -e --argjson expected "${expected}" '
            .items as $items
            | ($items | length == $expected)
              and ($items | all(.status.phase == "Running"))
          ' >/dev/null; then
      return
    fi
    kubectl get components -n "${TEST_NAMESPACE}" \
      -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=${sharding}" \
      -o wide
    sleep 5
  done

  kubectl get components -n "${TEST_NAMESPACE}" \
    -l "app.kubernetes.io/instance=${cluster},apps.kubeblocks.io/sharding-name=${sharding}" \
    -o wide
  fail "expected ${expected} Running shard components for ${cluster}/${sharding}"
}

wait_component_absent() {
  local name="$1"
  local cluster="${2:-}"

  log "waiting for Component ${name} to be removed"
  for _ in $(seq 1 120); do
    if ! kubectl get component "${name}" -n "${TEST_NAMESPACE}" >/dev/null 2>&1; then
      return
    fi
    if [[ -n "${cluster}" ]]; then
      print_component_remove_failures "${cluster}" "${name}"
    fi
    sleep 5
  done

  fail "Component ${name} still exists"
}

print_component_remove_failures() {
  local cluster="$1"
  local component="$2"
  local failures

  failures="$(
    kubectl get events -n "${TEST_NAMESPACE}" \
      --field-selector "involvedObject.kind=Cluster,involvedObject.name=${cluster}" \
      -o json \
      | jq -c --arg component "${component}" '
          [.items[]
           | select((.message // "") | contains($component))
           | select((.reason // "") == "ApplyResourcesFailed"
                    or ((.message // "") | test("timedOut|action timed-out|failed to call the shard remove action")))
           | {
               lastTimestamp: (.lastTimestamp // .eventTime // .metadata.creationTimestamp),
               reason: .reason,
               message: .message
             }]
          | sort_by(.lastTimestamp)
          | .[-3:]
        '
  )"
  if [[ "${failures}" != "[]" ]]; then
    printf '%s\n' "${failures}" | jq .
  fi
}

create_network_dns_cluster() {
  wait_component_definition_prefix "${MYSQL_COMPDEF}" >/dev/null
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-dns >/dev/null 2>&1; then
    log "cluster network-dns already exists; skipping create"
  else
    local dns_ip
    dns_ip="$(kubectl -n kube-system get svc kube-dns -o jsonpath='{.spec.clusterIP}' 2>/dev/null || true)"
    dns_ip="${dns_ip:-10.96.0.10}"

    create_manifest network-dns.yaml DNS_IP "${dns_ip}"
  fi

  wait_cluster_running network-dns
  assert_mysql_standalone_cluster network-dns
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
  wait_component_definition_prefix "${MYSQL_COMPDEF}" >/dev/null
  local port_name
  port_name="$(first_runtime_port_name "${MYSQL_COMPDEF}")"
  [[ -n "${port_name}" ]] || fail "ComponentDefinition ${MYSQL_COMPDEF} has no runtime container port name"

  if kubectl -n "${TEST_NAMESPACE}" get cluster network-hostport >/dev/null 2>&1; then
    log "cluster network-hostport already exists; skipping create"
  else
    create_manifest network-hostport.yaml PORT_NAME "${port_name}"
  fi

  wait_cluster_running network-hostport
  assert_mysql_standalone_cluster network-hostport
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

assert_pod_hostnetwork() {
  local cluster="$1"
  local expected="$2"
  local out="$3"
  cluster_pod_json_by_instance "${cluster}" "${out}"
  jq -e --argjson expected "${expected}" '
    .items | length == 1
    and (.[0].spec.hostNetwork == $expected)
    and (if $expected then (.[0].spec.dnsPolicy == "ClusterFirstWithHostNet" or .[0].spec.dnsPolicy == "Default") else true end)
  ' "${out}" >/dev/null
}

create_mongodb_hostnetwork_annotation_cluster() {
  wait_component_definition_prefix "${MONGO_COMPDEF_PREFIX}" >/dev/null
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-mongodb-hostnet-annotation >/dev/null 2>&1; then
    log "cluster network-mongodb-hostnet-annotation already exists; skipping create"
  else
    create_manifest network-mongodb-hostnet-annotation.yaml
  fi

  wait_cluster_running network-mongodb-hostnet-annotation
  assert_mongodb_standalone_cluster network-mongodb-hostnet-annotation
  assert_pod_hostnetwork network-mongodb-hostnet-annotation true "${WORK_DIR}/network-mongodb-hostnet-annotation-pods.json"
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-mongodb-hostnet-annotation -o json \
    | jq '.items[] | {name: .metadata.name, node: .spec.nodeName, hostIP: .status.hostIP, podIP: .status.podIP, hostNetwork: .spec.hostNetwork, dnsPolicy: .spec.dnsPolicy, ports: [.spec.containers[]?.ports[]?]}' \
    >"${WORK_DIR}/network-mongodb-hostnet-annotation.result.json"
}

create_mongodb_hostnetwork_api_cluster() {
  wait_component_definition_prefix "${MONGO_COMPDEF_PREFIX}" >/dev/null
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-mongodb-hostnet-api >/dev/null 2>&1; then
    log "cluster network-mongodb-hostnet-api already exists; skipping create"
  else
    create_manifest network-mongodb-hostnet-api.yaml
  fi

  wait_cluster_running network-mongodb-hostnet-api
  assert_mongodb_standalone_cluster network-mongodb-hostnet-api
  assert_pod_hostnetwork network-mongodb-hostnet-api true "${WORK_DIR}/network-mongodb-hostnet-api-pods.json"
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-mongodb-hostnet-api -o json \
    | jq '.items[] | {name: .metadata.name, node: .spec.nodeName, hostIP: .status.hostIP, podIP: .status.podIP, hostNetwork: .spec.hostNetwork, dnsPolicy: .spec.dnsPolicy, ports: [.spec.containers[]?.ports[]?]}' \
    >"${WORK_DIR}/network-mongodb-hostnet-api.result.json"
}

create_mysql_hostnetwork_negative_cluster() {
  wait_component_definition_prefix "${MYSQL_COMPDEF}" >/dev/null
  if kubectl -n "${TEST_NAMESPACE}" get cluster network-mysql-hostnet-negative >/dev/null 2>&1; then
    log "cluster network-mysql-hostnet-negative already exists; skipping create"
  else
    create_manifest network-mysql-hostnet-negative.yaml
  fi

  wait_cluster_running network-mysql-hostnet-negative
  assert_mysql_standalone_cluster network-mysql-hostnet-negative
  assert_pod_hostnetwork network-mysql-hostnet-negative false "${WORK_DIR}/network-mysql-hostnet-negative-pods.json"
  kubectl get pod -n "${TEST_NAMESPACE}" -l app.kubernetes.io/instance=network-mysql-hostnet-negative -o json \
    | jq '.items[] | {name: .metadata.name, node: .spec.nodeName, hostIP: .status.hostIP, podIP: .status.podIP, hostNetwork: .spec.hostNetwork, dnsPolicy: .spec.dnsPolicy, ports: [.spec.containers[]?.ports[]?]}' \
    >"${WORK_DIR}/network-mysql-hostnet-negative.result.json"
  kubectl -n "${KB_NAMESPACE}" get configmap -o yaml >"${WORK_DIR}/network-hostport-configmaps.yaml"
}

run_mongodb_sharding_rollout() {
  create_mongodb_sharding_cluster
  seed_mongodb_sharding_data mongo-sharding
  delete_rollout_if_exists mongodb-sharding-rollout
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-before-rollout-pods.json"

  create_manifest mongodb-sharding-rollout.yaml
  wait_rollout_succeed mongodb-sharding-rollout
  wait_cluster_running mongo-sharding
  snapshot_sharding_pods mongo-sharding shard "${WORK_DIR}/mongodb-sharding-after-rollout-pods.json"
  assert_sharding_rollout_status mongodb-sharding-rollout shard "${MONGO_SHARD_VERSION_BASE}"
  verify_mongodb_sharding_data mongo-sharding after-rollout
  assert_min_ready_interval "${WORK_DIR}/mongodb-sharding-after-rollout-pods.json" "${ROLLOUT_INSTANCE_INTERVAL_SECONDS}"
  assert_sharding_pods_replaced_with_version \
    "${WORK_DIR}/mongodb-sharding-before-rollout-pods.json" \
    "${WORK_DIR}/mongodb-sharding-after-rollout-pods.json" \
    "${MONGO_SHARD_VERSION_TARGET}"
  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MONGO_SHARD_VERSION_TARGET}" '
        .spec.shardings[]
        | select(.name == "shard")
        | .template.serviceVersion == $version
      ' >/dev/null
  kubectl get rollout mongodb-sharding-rollout -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/mongodb-sharding-rollout.result.yaml"
  kubectl get cluster mongo-sharding -n "${TEST_NAMESPACE}" -o yaml \
    >"${WORK_DIR}/mongodb-sharding.after-rollout.yaml"
}

run_case_upgrade() {
  create_or_select_kind
  install_kubeblocks "${SOURCE_VERSION}"
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_base_cluster
  upgrade_to_target_kubeblocks "${TARGET_VERSION}"
}

run_case_network() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_network_dns_cluster
  create_network_runtime_hostport_cluster
  create_mongodb_hostnetwork_annotation_cluster
  create_mongodb_hostnetwork_api_cluster
  create_mysql_hostnetwork_negative_cluster
}

run_case_rollout_api() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_test_cluster
  # run_mysql_inplace_rollout
  # run_mysql_replace_rollout
  run_mysql_create_rollout

  # run_mongodb_sharding_rollout
}

run_case_dynamic_instance_template() {
  ensure_namespace "${TEST_NAMESPACE}"
  create_mysql_test_cluster
  if ! kubectl get cluster "${MYSQL_CLUSTER}" -n "${TEST_NAMESPACE}" -o json \
    | jq -e --arg version "${MYSQL_VERSION_CANARY}" \
      '.spec.componentSpecs[] | select(.name == "mysql") | .instances[]? | select(.serviceVersion == $version)' >/dev/null; then
    run_mysql_create_rollout
  fi
  run_dynamic_mysql_instance_template_test
}

run_case_sharding() {
  ensure_namespace "${TEST_NAMESPACE}"
  run_mongodb_sharding_scale
  run_mongodb_heterogeneous_test
}

cleanup() {
  if kubectl get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
    kubectl delete namespace "${TEST_NAMESPACE}" --wait=true
  fi
}

run_stage() {
  case "$1" in
    setup-kind) preflight; create_or_select_kind ;;
    setup-cache) preflight; prepare_release_cache ;;
    setup-kb) preflight; install_kubeblocks "${TARGET_VERSION}" ;;
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
