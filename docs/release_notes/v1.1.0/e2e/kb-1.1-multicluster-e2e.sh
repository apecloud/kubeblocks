#!/usr/bin/env bash

set -euo pipefail

# Reproduce KubeBlocks 1.1 multi-cluster validation with local kind clusters:
#   control cluster: runs the KubeBlocks control plane and owns addons/ComponentDefinitions
#   data-a/data-b: run Instance-owned runtime objects only; no addons are installed there
#
# The script uses kubectl create/replace for CRs and CRDs. It avoids kubectl
# apply so the release validation path matches explicit create/replace flows.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"
WORK_DIR="${WORK_DIR:-/tmp/kb11-multicluster-e2e}"

KB_NAMESPACE="${KB_NAMESPACE:-kb-system}"
TEST_NAMESPACE="${TEST_NAMESPACE:-kb-11-mc-e2e}"
TARGET_VERSION="${TARGET_VERSION:-1.1.0-beta.6}"
AUTO_INSTALLED_ADDONS="${AUTO_INSTALLED_ADDONS:-[\"etcd\"]}"

CONTROL_CLUSTER="${CONTROL_CLUSTER:-kb11-mc-control}"
DATA_CLUSTER_A="${DATA_CLUSTER_A:-kb11-mc-data-a}"
DATA_CLUSTER_B="${DATA_CLUSTER_B:-kb11-mc-data-b}"
CONTROL_CONTEXT="${CONTROL_CONTEXT:-kind-${CONTROL_CLUSTER}}"
DATA_KUBE_CONTEXT_A="${DATA_KUBE_CONTEXT_A:-kind-${DATA_CLUSTER_A}}"
DATA_KUBE_CONTEXT_B="${DATA_KUBE_CONTEXT_B:-kind-${DATA_CLUSTER_B}}"
DATA_CONTEXT_A="${DATA_CONTEXT_A:-data-a}"
DATA_CONTEXT_B="${DATA_CONTEXT_B:-data-b}"
MC_SECRET_NAME="${MC_SECRET_NAME:-kubeblocks-multicluster-kubeconfig}"

KIND_DOCKER_NETWORK="${KIND_DOCKER_NETWORK:-kb-mc-kind}"
CONTROL_POD_SUBNET="${CONTROL_POD_SUBNET:-10.30.0.0/16}"
CONTROL_SERVICE_SUBNET="${CONTROL_SERVICE_SUBNET:-10.130.0.0/16}"
DATA_A_POD_SUBNET="${DATA_A_POD_SUBNET:-10.10.0.0/16}"
DATA_A_SERVICE_SUBNET="${DATA_A_SERVICE_SUBNET:-10.110.0.0/16}"
DATA_B_POD_SUBNET="${DATA_B_POD_SUBNET:-10.20.0.0/16}"
DATA_B_SERVICE_SUBNET="${DATA_B_SERVICE_SUBNET:-10.120.0.0/16}"

CILIUM_VERSION="${CILIUM_VERSION:-1.19.4}"
CILIUM_ENVOY_VERSION="${CILIUM_ENVOY_VERSION:-v1.36.6-1778235340-b87d1e32f522b33bd51701c6476d199326f01496}"
CILIUM_CLUSTER_A_ID="${CILIUM_CLUSTER_A_ID:-1}"
CILIUM_CLUSTER_B_ID="${CILIUM_CLUSTER_B_ID:-2}"
CILIUM_CLUSTER_A_NAME="${CILIUM_CLUSTER_A_NAME:-${DATA_CONTEXT_A}}"
CILIUM_CLUSTER_B_NAME="${CILIUM_CLUSTER_B_NAME:-${DATA_CONTEXT_B}}"
CILIUM_CLUSTERMESH_NODEPORT="${CILIUM_CLUSTERMESH_NODEPORT:-32379}"
PRELOAD_CILIUM_IMAGES="${PRELOAD_CILIUM_IMAGES:-true}"
VERIFY_POD_NETWORK="${VERIFY_POD_NETWORK:-true}"
NETWORK_TEST_NAMESPACE="${NETWORK_TEST_NAMESPACE:-mc-net-test}"
NETWORK_TEST_IMAGE="${NETWORK_TEST_IMAGE:-busybox:1.36.1}"

ETCD_CLUSTER="${ETCD_CLUSTER:-etcd-mc}"
ETCD_COMPDEF="${ETCD_COMPDEF:-etcd-3-1.1.0-alpha.0}"
ETCD_SERVICE_VERSION="${ETCD_SERVICE_VERSION:-3.6.1}"

RESET_KIND="${RESET_KIND:-false}"
RESET_TEST_NAMESPACE="${RESET_TEST_NAMESPACE:-false}"
SKIP_HELM_REPO_UPDATE="${SKIP_HELM_REPO_UPDATE:-false}"

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
  bash docs/release_notes/v1.1.0/e2e/kb-1.1-multicluster-e2e.sh [stage ...]

Stages:
  all             Run kind setup, KubeBlocks install, etcd multi-cluster test.
  kind            Create or select three kind clusters.
  install         Install KubeBlocks 1.1 into control and data clusters.
                  Only the control cluster installs addons; data clusters use autoInstalledAddons=[].
  create-secret   Generate data-cluster kubeconfig and create the control secret.
  etcd            Create the multi-cluster etcd Cluster and verify placement.
  disable-scale   Disable data-b, scale etcd from 2 to 3, verify new placement.
  cleanup         Delete test namespace in all clusters.
  delete-kind     Delete the three kind clusters.
  teardown        Delete kind clusters, the shared Docker network, and temporary files.

Useful env vars:
  TARGET_VERSION=1.1.0-beta.6
  CONTROL_CLUSTER=kb11-mc-control
  DATA_CLUSTER_A=kb11-mc-data-a
  DATA_CLUSTER_B=kb11-mc-data-b
  DATA_CONTEXT_A=data-a
  DATA_CONTEXT_B=data-b
  KIND_DOCKER_NETWORK=kb-mc-kind
  CILIUM_VERSION=1.19.4
  PRELOAD_CILIUM_IMAGES=true|false
  VERIFY_POD_NETWORK=true|false
  ETCD_COMPDEF=etcd
  ETCD_SERVICE_VERSION=3.6.1
  RESET_KIND=true|false
  RESET_TEST_NAMESPACE=true|false
  WORK_DIR=/tmp/kb11-multicluster-e2e

Examples:
  RESET_KIND=true bash docs/release_notes/v1.1.0/e2e/kb-1.1-multicluster-e2e.sh all
  bash docs/release_notes/v1.1.0/e2e/kb-1.1-multicluster-e2e.sh etcd disable-scale
USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

preflight() {
  require_cmd docker
  require_cmd helm
  require_cmd jq
  require_cmd kind
  require_cmd kubectl
  mkdir -p "${WORK_DIR}"
  log "repo root: ${ROOT_DIR}"
  log "work dir: ${WORK_DIR}"
  log "control kube context: ${CONTROL_CONTEXT}"
  log "data kube contexts: ${DATA_KUBE_CONTEXT_A}, ${DATA_KUBE_CONTEXT_B}"
  log "placement aliases mounted into control manager: ${DATA_CONTEXT_A}, ${DATA_CONTEXT_B}"
  log "kind docker network: ${KIND_DOCKER_NETWORK}"
  log "data pod CIDRs: ${DATA_A_POD_SUBNET}, ${DATA_B_POD_SUBNET}"
  log "control autoInstalledAddons: ${AUTO_INSTALLED_ADDONS}"
  log "data autoInstalledAddons: []"
  log "etcd target: ${ETCD_COMPDEF} / ${ETCD_SERVICE_VERSION}"
}

kind_context() {
  printf 'kind-%s' "$1"
}

kind_control_plane_container() {
  printf '%s-control-plane' "$1"
}

create_kind_cluster() {
  local cluster="$1"
  local pod_subnet="$2"
  local service_subnet="$3"
  local disable_default_cni="$4"
  if kind get clusters | grep -qx "${cluster}"; then
    if [[ "${RESET_KIND}" == "true" ]]; then
      log "deleting existing kind cluster ${cluster}"
      kind delete cluster --name "${cluster}"
    else
      log "kind cluster ${cluster} already exists"
      return
    fi
  fi

  cat >"${WORK_DIR}/${cluster}.kind.yaml" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  podSubnet: "${pod_subnet}"
  serviceSubnet: "${service_subnet}"
  disableDefaultCNI: ${disable_default_cni}
nodes:
  - role: control-plane
  - role: worker
EOF
  log "creating kind cluster ${cluster}"
  env -u HTTP_PROXY -u HTTPS_PROXY -u http_proxy -u https_proxy \
    KIND_EXPERIMENTAL_DOCKER_NETWORK="${KIND_DOCKER_NETWORK}" \
    kind create cluster --name "${cluster}" --config "${WORK_DIR}/${cluster}.kind.yaml"
}

ensure_kind_docker_network() {
  if docker network inspect "${KIND_DOCKER_NETWORK}" >/dev/null 2>&1; then
    log "docker network ${KIND_DOCKER_NETWORK} already exists"
  else
    log "creating docker network ${KIND_DOCKER_NETWORK}"
    docker network create "${KIND_DOCKER_NETWORK}"
  fi
}

ensure_cilium_repo() {
  if ! helm repo list -o json | jq -e '.[] | select(.name == "cilium")' >/dev/null; then
    helm repo add cilium https://helm.cilium.io
  fi
  if [[ "${SKIP_HELM_REPO_UPDATE}" != "true" ]]; then
    helm repo update cilium
  fi
}

load_image_to_data_clusters() {
  local image="$1"
  log "preloading ${image} into ${DATA_CLUSTER_A} and ${DATA_CLUSTER_B}"
  docker pull "${image}"
  kind load docker-image "${image}" --name "${DATA_CLUSTER_A}"
  kind load docker-image "${image}" --name "${DATA_CLUSTER_B}"
}

preload_cilium_images() {
  [[ "${PRELOAD_CILIUM_IMAGES}" == "true" ]] || return
  load_image_to_data_clusters "quay.io/cilium/cilium:v${CILIUM_VERSION}"
  load_image_to_data_clusters "quay.io/cilium/operator-generic:v${CILIUM_VERSION}"
  load_image_to_data_clusters "quay.io/cilium/cilium-envoy:${CILIUM_ENVOY_VERSION}"
  load_image_to_data_clusters "quay.io/cilium/clustermesh-apiserver:v${CILIUM_VERSION}"
}

install_cilium() {
  local context="$1"
  local cluster_name="$2"
  local cluster_id="$3"

  log "installing Cilium ${CILIUM_VERSION} in ${context}"
  helm upgrade --install cilium cilium/cilium \
    --version "${CILIUM_VERSION}" \
    --namespace kube-system \
    --kube-context "${context}" \
    --set "cluster.name=${cluster_name}" \
    --set "cluster.id=${cluster_id}" \
    --set ipam.mode=kubernetes \
    --set kubeProxyReplacement=false \
    --set clustermesh.useAPIServer=true \
    --set clustermesh.config.enabled=true \
    --set clustermesh.apiserver.service.type=NodePort \
    --set "clustermesh.apiserver.service.nodePort=${CILIUM_CLUSTERMESH_NODEPORT}" \
    --set image.useDigest=false \
    --set envoy.image.useDigest=false \
    --set operator.image.useDigest=false \
    --set clustermesh.apiserver.image.useDigest=false \
    --set certgen.image.useDigest=false \
    --set hubble.relay.image.useDigest=false
}

wait_cilium_ready() {
  local context="$1"
  log "waiting for Cilium in ${context}"
  kubectl --context "${context}" -n kube-system rollout status daemonset/cilium --timeout=10m
  kubectl --context "${context}" -n kube-system rollout status deploy/cilium-operator --timeout=10m
  kubectl --context "${context}" -n kube-system rollout status deploy/clustermesh-apiserver --timeout=10m
  cilium status --context "${context}" --wait
}

connect_clustermesh() {
  log "connecting Cilium Cluster Mesh ${DATA_KUBE_CONTEXT_A} <=> ${DATA_KUBE_CONTEXT_B}"
  cilium clustermesh connect \
    --context "${DATA_KUBE_CONTEXT_A}" \
    --destination-context "${DATA_KUBE_CONTEXT_B}" \
    --connection-mode bidirectional \
    --allow-mismatching-ca
  cilium clustermesh status --context "${DATA_KUBE_CONTEXT_A}" --wait
  cilium clustermesh status --context "${DATA_KUBE_CONTEXT_B}" --wait
}

verify_cross_cluster_pod_network() {
  [[ "${VERIFY_POD_NETWORK}" == "true" ]] || return
  load_image_to_data_clusters "${NETWORK_TEST_IMAGE}"

  for context in "${DATA_KUBE_CONTEXT_A}" "${DATA_KUBE_CONTEXT_B}"; do
    ensure_namespace "${context}" "${NETWORK_TEST_NAMESPACE}"
  done

  kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${NETWORK_TEST_NAMESPACE}" delete pod net-a --ignore-not-found --wait=true
  kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${NETWORK_TEST_NAMESPACE}" delete pod net-b --ignore-not-found --wait=true
  kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${NETWORK_TEST_NAMESPACE}" run net-a \
    --image="${NETWORK_TEST_IMAGE}" --image-pull-policy=IfNotPresent --command -- sleep 3600
  kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${NETWORK_TEST_NAMESPACE}" run net-b \
    --image="${NETWORK_TEST_IMAGE}" --image-pull-policy=IfNotPresent --command -- sleep 3600
  kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${NETWORK_TEST_NAMESPACE}" wait --for=condition=Ready pod/net-a --timeout=5m
  kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${NETWORK_TEST_NAMESPACE}" wait --for=condition=Ready pod/net-b --timeout=5m

  local pod_a_ip pod_b_ip
  pod_a_ip="$(kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${NETWORK_TEST_NAMESPACE}" get pod net-a -o jsonpath='{.status.podIP}')"
  pod_b_ip="$(kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${NETWORK_TEST_NAMESPACE}" get pod net-b -o jsonpath='{.status.podIP}')"
  log "verifying cross-cluster Pod network: ${pod_a_ip} <=> ${pod_b_ip}"
  kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${NETWORK_TEST_NAMESPACE}" exec net-a -- ping -c 3 "${pod_b_ip}"
  kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${NETWORK_TEST_NAMESPACE}" exec net-b -- ping -c 3 "${pod_a_ip}"
}

create_or_select_kind_clusters() {
  require_cmd cilium
  ensure_kind_docker_network
  create_kind_cluster "${CONTROL_CLUSTER}" "${CONTROL_POD_SUBNET}" "${CONTROL_SERVICE_SUBNET}" "false"
  create_kind_cluster "${DATA_CLUSTER_A}" "${DATA_A_POD_SUBNET}" "${DATA_A_SERVICE_SUBNET}" "true"
  create_kind_cluster "${DATA_CLUSTER_B}" "${DATA_B_POD_SUBNET}" "${DATA_B_SERVICE_SUBNET}" "true"
  kubectl config use-context "${CONTROL_CONTEXT}"
  kubectl --context "${CONTROL_CONTEXT}" cluster-info
  kubectl --context "${DATA_KUBE_CONTEXT_A}" cluster-info
  kubectl --context "${DATA_KUBE_CONTEXT_B}" cluster-info
  ensure_cilium_repo
  preload_cilium_images
  install_cilium "${DATA_KUBE_CONTEXT_A}" "${CILIUM_CLUSTER_A_NAME}" "${CILIUM_CLUSTER_A_ID}"
  install_cilium "${DATA_KUBE_CONTEXT_B}" "${CILIUM_CLUSTER_B_NAME}" "${CILIUM_CLUSTER_B_ID}"
  wait_cilium_ready "${DATA_KUBE_CONTEXT_A}"
  wait_cilium_ready "${DATA_KUBE_CONTEXT_B}"
  connect_clustermesh
  verify_cross_cluster_pod_network
}

ensure_namespace() {
  local context="$1"
  local namespace="$2"
  if kubectl --context "${context}" get namespace "${namespace}" >/dev/null 2>&1; then
    log "namespace ${namespace} already exists in ${context}"
  else
    kubectl --context "${context}" create namespace "${namespace}"
  fi
}

ensure_test_namespaces() {
  local context
  for context in "${CONTROL_CONTEXT}" "${DATA_KUBE_CONTEXT_A}" "${DATA_KUBE_CONTEXT_B}"; do
    if [[ "${RESET_TEST_NAMESPACE}" == "true" ]] \
      && kubectl --context "${context}" get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
      kubectl --context "${context}" delete namespace "${TEST_NAMESPACE}" --wait=true
    fi
    ensure_namespace "${context}" "${TEST_NAMESPACE}"
  done
}

ensure_helm_repo() {
  if ! helm repo list -o json | jq -e '.[] | select(.name == "kubeblocks")' >/dev/null; then
    helm repo add kubeblocks https://apecloud.github.io/helm-charts
  fi
  if [[ "${SKIP_HELM_REPO_UPDATE}" != "true" ]]; then
    helm repo update kubeblocks
  fi
}

install_snapshot_crds() {
  local context="$1"
  if kubectl --context "${context}" get crd volumesnapshots.snapshot.storage.k8s.io >/dev/null 2>&1; then
    log "Snapshot CRDs already exist in ${context}"
  else
    kubectl --context "${context}" create -f "${ROOT_DIR}/deploy/helm/crds/snapshot/"
  fi
}

install_kubeblocks_crds() {
  local context="$1"
  if kubectl --context "${context}" get crd clusters.apps.kubeblocks.io >/dev/null 2>&1; then
    log "KubeBlocks CRDs already exist in ${context}; replacing with v${TARGET_VERSION}"
    kubectl --context "${context}" replace -f "https://github.com/apecloud/kubeblocks/releases/download/v${TARGET_VERSION}/kubeblocks_crds.yaml"
  else
    kubectl --context "${context}" create -f "https://github.com/apecloud/kubeblocks/releases/download/v${TARGET_VERSION}/kubeblocks_crds.yaml"
  fi
}

wait_deploy_if_exists() {
  local context="$1"
  local deploy="$2"
  if kubectl --context "${context}" -n "${KB_NAMESPACE}" get deploy "${deploy}" >/dev/null 2>&1; then
    kubectl --context "${context}" -n "${KB_NAMESPACE}" rollout status "deploy/${deploy}" --timeout=10m
  else
    log "deployment ${context}/${KB_NAMESPACE}/${deploy} does not exist; skipping"
  fi
}

wait_component_definition() {
  local context="$1"
  local name="$2"
  log "waiting for ComponentDefinition ${name} in ${context}"
  for _ in $(seq 1 120); do
    if kubectl --context "${context}" get componentdefinition "${name}" >/dev/null 2>&1; then
      return
    fi
    sleep 5
  done
  fail "ComponentDefinition ${name} did not appear in ${context}"
}

wait_instance_crd() {
  local context="$1"
  log "waiting for Instance CRD in ${context}"
  for _ in $(seq 1 60); do
    if kubectl --context "${context}" get crd instances.workloads.kubeblocks.io >/dev/null 2>&1; then
      return
    fi
    sleep 5
  done
  fail "Instance CRD did not appear in ${context}"
}

install_kubeblocks_on_data_cluster() {
  local context="$1"
  ensure_namespace "${context}" "${KB_NAMESPACE}"
  install_snapshot_crds "${context}"
  install_kubeblocks_crds "${context}"

  if helm --kube-context "${context}" -n "${KB_NAMESPACE}" status kubeblocks >/dev/null 2>&1; then
    log "upgrading data cluster ${context} KubeBlocks to ${TARGET_VERSION}"
    helm --kube-context "${context}" -n "${KB_NAMESPACE}" upgrade kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      --skip-crds \
      --reuse-values \
      --set-json "autoInstalledAddons=[]"
  else
    log "installing data cluster ${context} KubeBlocks ${TARGET_VERSION}"
    helm --kube-context "${context}" install kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      -n "${KB_NAMESPACE}" \
      --create-namespace \
      --skip-crds \
      --set-json "autoInstalledAddons=[]"
  fi
  wait_instance_crd "${context}"
  wait_deploy_if_exists "${context}" kubeblocks
  wait_deploy_if_exists "${context}" kubeblocks-dataprotection
}

docker_bridge_ip() {
  local cluster="$1"
  docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$(kind_control_plane_container "${cluster}")"
}

extract_context_auth() {
  local kind_ctx="$1"
  local user_name cert_data key_data
  user_name="$(kubectl config view --raw -o json | jq -r --arg ctx "${kind_ctx}" '.contexts[] | select(.name == $ctx).context.user')"
  cert_data="$(kubectl config view --raw -o json | jq -r --arg user "${user_name}" '.users[] | select(.name == $user).user["client-certificate-data"]')"
  key_data="$(kubectl config view --raw -o json | jq -r --arg user "${user_name}" '.users[] | select(.name == $user).user["client-key-data"]')"
  [[ -n "${cert_data}" && "${cert_data}" != "null" ]] || fail "missing client-certificate-data for context ${kind_ctx}"
  [[ -n "${key_data}" && "${key_data}" != "null" ]] || fail "missing client-key-data for context ${kind_ctx}"
  printf '%s\n%s\n' "${cert_data}" "${key_data}"
}

write_data_context() {
  local alias="$1"
  local kind_cluster="$2"
  local kind_ctx cert_data key_data api_ip
  kind_ctx="$(kind_context "${kind_cluster}")"
  api_ip="$(docker_bridge_ip "${kind_cluster}")"
  mapfile -t auth < <(extract_context_auth "${kind_ctx}")
  cert_data="${auth[0]}"
  key_data="${auth[1]}"

  cat >>"${WORK_DIR}/multicluster.kubeconfig" <<EOF
- cluster:
    insecure-skip-tls-verify: true
    server: https://${api_ip}:6443
  name: ${alias}
EOF
  cat >>"${WORK_DIR}/multicluster.contexts" <<EOF
- context:
    cluster: ${alias}
    user: ${alias}
  name: ${alias}
EOF
  cat >>"${WORK_DIR}/multicluster.users" <<EOF
- name: ${alias}
  user:
    client-certificate-data: ${cert_data}
    client-key-data: ${key_data}
EOF
}

write_multicluster_kubeconfig() {
  cat >"${WORK_DIR}/multicluster.kubeconfig" <<'EOF'
apiVersion: v1
kind: Config
preferences: {}
clusters:
EOF
  : >"${WORK_DIR}/multicluster.contexts"
  : >"${WORK_DIR}/multicluster.users"

  write_data_context "${DATA_CONTEXT_A}" "${DATA_CLUSTER_A}"
  write_data_context "${DATA_CONTEXT_B}" "${DATA_CLUSTER_B}"

  {
    printf 'contexts:\n'
    cat "${WORK_DIR}/multicluster.contexts"
    printf 'current-context: %s\n' "${DATA_CONTEXT_A}"
    printf 'users:\n'
    cat "${WORK_DIR}/multicluster.users"
  } >>"${WORK_DIR}/multicluster.kubeconfig"
}

create_multicluster_secret() {
  ensure_namespace "${CONTROL_CONTEXT}" "${KB_NAMESPACE}"
  write_multicluster_kubeconfig

  if kubectl --context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" get secret "${MC_SECRET_NAME}" >/dev/null 2>&1; then
    log "replacing multi-cluster kubeconfig secret ${MC_SECRET_NAME}"
    kubectl --context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" create secret generic "${MC_SECRET_NAME}" \
      --from-file=kubeconfig="${WORK_DIR}/multicluster.kubeconfig" \
      --dry-run=client -o yaml \
      | kubectl --context "${CONTROL_CONTEXT}" replace -f -
  else
    kubectl --context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" create secret generic "${MC_SECRET_NAME}" \
      --from-file=kubeconfig="${WORK_DIR}/multicluster.kubeconfig"
  fi
}

write_control_values() {
  local contexts_disabled="${1:-}"
  cat >"${WORK_DIR}/multicluster-control-values.yaml" <<EOF
autoInstalledAddons: ${AUTO_INSTALLED_ADDONS}
multiCluster:
  kubeConfig: ${MC_SECRET_NAME}
  mountPath: /var/run/secrets/kubeblocks.io/multicluster
  contexts: "${DATA_CONTEXT_A},${DATA_CONTEXT_B}"
  contextsDisabled: "${contexts_disabled}"
EOF
}

install_kubeblocks_on_control_cluster() {
  ensure_namespace "${CONTROL_CONTEXT}" "${KB_NAMESPACE}"
  install_snapshot_crds "${CONTROL_CONTEXT}"
  install_kubeblocks_crds "${CONTROL_CONTEXT}"
  create_multicluster_secret
  write_control_values ""

  if helm --kube-context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" status kubeblocks >/dev/null 2>&1; then
    log "upgrading control cluster KubeBlocks to ${TARGET_VERSION} with multiCluster enabled"
    helm --kube-context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" upgrade kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      --skip-crds \
      --reuse-values \
      -f "${WORK_DIR}/multicluster-control-values.yaml"
  else
    log "installing control cluster KubeBlocks ${TARGET_VERSION} with multiCluster enabled"
    helm --kube-context "${CONTROL_CONTEXT}" install kubeblocks kubeblocks/kubeblocks \
      --version "${TARGET_VERSION}" \
      -n "${KB_NAMESPACE}" \
      --create-namespace \
      --skip-crds \
      -f "${WORK_DIR}/multicluster-control-values.yaml"
  fi
  wait_deploy_if_exists "${CONTROL_CONTEXT}" kubeblocks
  wait_deploy_if_exists "${CONTROL_CONTEXT}" kubeblocks-dataprotection
  wait_component_definition "${CONTROL_CONTEXT}" "${ETCD_COMPDEF}"
}

install_kubeblocks() {
  ensure_helm_repo
  install_kubeblocks_on_control_cluster
  install_kubeblocks_on_data_cluster "${DATA_KUBE_CONTEXT_A}"
  install_kubeblocks_on_data_cluster "${DATA_KUBE_CONTEXT_B}"
}

wait_cluster_running() {
  local cluster="$1"
  kubectl --context "${CONTROL_CONTEXT}" wait "cluster/${cluster}" -n "${TEST_NAMESPACE}" \
    --for=jsonpath='{.status.phase}'=Running \
    --timeout=40m
}

wait_instance_count() {
  local expected="$1"
  for _ in $(seq 1 180); do
    local count
    count="$(kubectl --context "${CONTROL_CONTEXT}" -n "${TEST_NAMESPACE}" get instances.workloads.kubeblocks.io \
      -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" \
      -o json 2>/dev/null | jq '.items | length')"
    if [[ "${count}" == "${expected}" ]]; then
      return
    fi
    sleep 5
  done
  fail "expected ${expected} Instance objects for ${ETCD_CLUSTER}"
}

wait_data_pod_count() {
  local context="$1"
  local expected_min="$2"
  for _ in $(seq 1 180); do
    local count
    count="$(kubectl --context "${context}" -n "${TEST_NAMESPACE}" get pod \
      -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" \
      -o json 2>/dev/null | jq '.items | length')"
    if (( count >= expected_min )); then
      return
    fi
    sleep 5
  done
  fail "expected at least ${expected_min} pod(s) for ${ETCD_CLUSTER} in ${context}"
}

create_etcd_multicluster() {
  ensure_test_namespaces
  if kubectl --context "${CONTROL_CONTEXT}" -n "${TEST_NAMESPACE}" get cluster "${ETCD_CLUSTER}" >/dev/null 2>&1; then
    log "cluster ${ETCD_CLUSTER} already exists; skipping create"
  else
    cat >"${WORK_DIR}/etcd-multicluster.yaml" <<EOF
apiVersion: apps.kubeblocks.io/v1
kind: Cluster
metadata:
  name: ${ETCD_CLUSTER}
  namespace: ${TEST_NAMESPACE}
  annotations:
    apps.kubeblocks.io/multi-cluster-placement: ${DATA_CONTEXT_A},${DATA_CONTEXT_B}
spec:
  terminationPolicy: Delete
  componentSpecs:
    - name: etcd
      componentDef: ${ETCD_COMPDEF}
      serviceVersion: "${ETCD_SERVICE_VERSION}"
      enableInstanceAPI: true
      replicas: 3
      resources:
        requests:
          cpu: 500m
          memory: 512Mi
        limits:
          cpu: 500m
          memory: 512Mi
      volumeClaimTemplates:
        - name: data
          spec:
            accessModes:
              - ReadWriteOnce
            resources:
              requests:
                storage: 10Gi
EOF
    kubectl --context "${CONTROL_CONTEXT}" create -f "${WORK_DIR}/etcd-multicluster.yaml"
  fi

  wait_cluster_running "${ETCD_CLUSTER}"
  wait_instance_count 2
  wait_data_pod_count "${DATA_KUBE_CONTEXT_A}" 1
  wait_data_pod_count "${DATA_KUBE_CONTEXT_B}" 1
  verify_multicluster_placement 2
}

verify_multicluster_placement() {
  local expected_replicas="$1"
  kubectl --context "${CONTROL_CONTEXT}" -n "${TEST_NAMESPACE}" get instances.workloads.kubeblocks.io \
    -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" \
    -o json >"${WORK_DIR}/etcd-mc-instances.json"
  kubectl --context "${CONTROL_CONTEXT}" -n "${TEST_NAMESPACE}" get cluster "${ETCD_CLUSTER}" -o yaml \
    >"${WORK_DIR}/etcd-mc.cluster.yaml"
  kubectl --context "${DATA_KUBE_CONTEXT_A}" -n "${TEST_NAMESPACE}" get pod,pvc \
    -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" -o wide \
    >"${WORK_DIR}/etcd-mc.${DATA_CONTEXT_A}.runtime.txt" || true
  kubectl --context "${DATA_KUBE_CONTEXT_B}" -n "${TEST_NAMESPACE}" get pod,pvc \
    -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" -o wide \
    >"${WORK_DIR}/etcd-mc.${DATA_CONTEXT_B}.runtime.txt" || true

  jq -e --arg a "${DATA_CONTEXT_A}" --arg b "${DATA_CONTEXT_B}" --argjson expected "${expected_replicas}" '
    (.items | length) == $expected
    and ([.items[].metadata.annotations["apps.kubeblocks.io/multi-cluster-placement"]] | all(. == $a or . == $b))
  ' "${WORK_DIR}/etcd-mc-instances.json" >/dev/null

  log "Instance placement:"
  jq -r '.items[] | [.metadata.name, .metadata.annotations["apps.kubeblocks.io/multi-cluster-placement"]] | @tsv' \
    "${WORK_DIR}/etcd-mc-instances.json"
}

delete_one_data_pod_and_wait_repair() {
  local victim_context="${DATA_KUBE_CONTEXT_A}"
  local victim
  victim="$(kubectl --context "${victim_context}" -n "${TEST_NAMESPACE}" get pod \
    -l "app.kubernetes.io/instance=${ETCD_CLUSTER}" \
    -o jsonpath='{.items[0].metadata.name}')"
  [[ -n "${victim}" ]] || fail "no pod found in ${victim_context} for repair test"
  log "deleting ${victim_context}/${TEST_NAMESPACE}/${victim} and waiting for repair"
  kubectl --context "${victim_context}" -n "${TEST_NAMESPACE}" delete pod "${victim}" --wait=true
  wait_data_pod_count "${victim_context}" 1
}

disable_data_b_and_scale() {
  log "disabling ${DATA_CONTEXT_B} in control Helm values"
  write_control_values "${DATA_CONTEXT_B}"
  helm --kube-context "${CONTROL_CONTEXT}" -n "${KB_NAMESPACE}" upgrade kubeblocks kubeblocks/kubeblocks \
    --version "${TARGET_VERSION}" \
    --skip-crds \
    --reuse-values \
    -f "${WORK_DIR}/multicluster-control-values.yaml"
  wait_deploy_if_exists "${CONTROL_CONTEXT}" kubeblocks

  kubectl --context "${CONTROL_CONTEXT}" -n "${TEST_NAMESPACE}" get cluster "${ETCD_CLUSTER}" -o json \
    | jq '(.spec.componentSpecs[] | select(.name == "etcd").replicas) = 3' \
    | kubectl --context "${CONTROL_CONTEXT}" replace -f -

  wait_cluster_running "${ETCD_CLUSTER}"
  wait_instance_count 3
  verify_multicluster_placement 3
  jq -e --arg disabled "${DATA_CONTEXT_B}" '
    [.items[] | select((.metadata.name | test("-2$")) or (.spec.ordinal == 2))][0]
    | .metadata.annotations["apps.kubeblocks.io/multi-cluster-placement"] != $disabled
  ' "${WORK_DIR}/etcd-mc-instances.json" >/dev/null
}

run_etcd_multicluster_test() {
  create_etcd_multicluster
  delete_one_data_pod_and_wait_repair
}

cleanup() {
  local context
  for context in "${CONTROL_CONTEXT}" "${DATA_KUBE_CONTEXT_A}" "${DATA_KUBE_CONTEXT_B}"; do
    if kubectl --context "${context}" get namespace "${TEST_NAMESPACE}" >/dev/null 2>&1; then
      kubectl --context "${context}" delete namespace "${TEST_NAMESPACE}" --wait=true
    fi
  done
  for context in "${DATA_KUBE_CONTEXT_A}" "${DATA_KUBE_CONTEXT_B}"; do
    if kubectl --context "${context}" get namespace "${NETWORK_TEST_NAMESPACE}" >/dev/null 2>&1; then
      kubectl --context "${context}" delete namespace "${NETWORK_TEST_NAMESPACE}" --wait=true
    fi
  done
}

delete_kind_clusters() {
  kind delete cluster --name "${CONTROL_CLUSTER}" || true
  kind delete cluster --name "${DATA_CLUSTER_A}" || true
  kind delete cluster --name "${DATA_CLUSTER_B}" || true
}

delete_kind_docker_network() {
  if docker network inspect "${KIND_DOCKER_NETWORK}" >/dev/null 2>&1; then
    log "deleting docker network ${KIND_DOCKER_NETWORK}"
    docker network rm "${KIND_DOCKER_NETWORK}" || true
  else
    log "docker network ${KIND_DOCKER_NETWORK} does not exist"
  fi
}

teardown_environment() {
  delete_kind_clusters
  delete_kind_docker_network
  if [[ -d "${WORK_DIR}" ]]; then
    log "deleting work dir ${WORK_DIR}"
    rm -rf "${WORK_DIR}"
  fi
}

run_all() {
  preflight
  create_or_select_kind_clusters
  create_multicluster_secret
  install_kubeblocks
  run_etcd_multicluster_test
  disable_data_b_and_scale
}

run_stage() {
  case "$1" in
    all) run_all ;;
    kind) preflight; create_or_select_kind_clusters ;;
    install) preflight; install_kubeblocks ;;
    create-secret) preflight; create_multicluster_secret ;;
    etcd) preflight; run_etcd_multicluster_test ;;
    disable-scale) preflight; disable_data_b_and_scale ;;
    cleanup) preflight; cleanup ;;
    delete-kind) preflight; delete_kind_clusters ;;
    teardown) preflight; teardown_environment ;;
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
