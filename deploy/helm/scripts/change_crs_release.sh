#!/bin/bash
release=$1
namespace=$2

function updateRelease() {
    local kind=$1
    local name=$2
    kubectl annotate $kind $name --overwrite meta.helm.sh/release-name=$release
    kubectl annotate $kind $name --overwrite meta.helm.sh/release-namespace=$namespace
}

# 1. change clusterRoles
clusterRoles=(
    "cluster-editor-role"
    "clusterdefinition-editor-role"
    "configconstraint-editor-role"
    "metrics-reader"
    "proxy-role"
    "patroni-pod-role"
    "manager-role"
    "dataprotection-worker-role"
    "helmhook-role"
    "backup-editor-role"
    "backuppolicy-editor-role"
    "dataprotection-exec-worker-role"
    "restore-editor-role"
    "nodecountscaler-editor-role"
    "editor-role"
    "leader-election-role"
    "rbac-manager-role"
    "instanceset-editor-role"
)

for role in "${clusterRoles[@]}"; do
    updateRelease ClusterRole "${release}-${role}"
done

# 2. change addons
addons=(
    "apecloud-mysql"
    "etcd"
    "kafka"
    "llm"
    "mongodb"
    "mysql"
    "postgresql"
    "pulsar"
    "qdrant"
    "redis"
    "alertmanager-webhook-adaptor"
    "aws-load-balancer-controller"
    "csi-driver-nfs"
    "csi-hostpath-driver"
    "grafana"
    "prometheus"
    "snapshot-controller"
    "victoria-metrics-agent"
)

for addon in "${addons[@]}"; do
    updateRelease Addon "$addon"
done

# 3. change storageProvider
storageProviders=(
    "cos"
    "ftp"
    "gcs-s3comp"
    "minio"
    "nfs"
    "obs"
    "oss"
    "pvc"
    "s3"
)

for sp in "${storageProviders[@]}"; do
    updateRelease StorageProviders "$sp"
done

# 4. change backupRepo
updateRelease BackupRepo ${release}-backuprepo