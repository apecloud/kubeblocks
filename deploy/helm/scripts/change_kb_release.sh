#!/bin/bash
release=
namespace=

for i in "$@"; do
  case $i in
    --release-name=*)
      release="${i#*=}"
      shift
      ;;
    --namespace=*)
      namespace="${i#*=}"
      shift
      ;;
    *)
      echo "Unknown option $i"
      exit 1
      ;;
  esac
done
if [ "$release" == "" ] || [ "$namespace" == "" ]; then
  echo "--release-name and --namespace are required"
  exit 1
fi
echo "KubeBlocks release name: $release, namespace: $namespace"


function takeOverResources() {
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
    takeOverResources ClusterRole "${release}-${role}"
done
takeOverResources ClusterRole "${release}"
takeOverResources ClusterRole "kubeblocks-cluster-pod-role"

# 2. change addons
addons=(
    "apecloud-mysql"
    "etcd"
    "kafka"
    "mongodb"
    "mysql"
    "postgresql"
    "qdrant"
    "redis"
)

for addon in "${addons[@]}"; do
    takeOverResources Addon "$addon"
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
    takeOverResources StorageProviders "$sp"
done

# 4. change backupRepo
takeOverResources BackupRepo ${release}-backuprepo

# 5. takeover StorageClass
takeOverResources StorageClass kb-default-sc
