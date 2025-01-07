#!/bin/bash

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
    --addon-name=*)
      addon="${i#*=}"
      shift
      ;;
    *)
      echo "Unknown option $i"
      exit 1
      ;;
  esac
done
if [ "$release" == "" ] || [ "$namespace" == "" ] || [ "$addon" == "" ]; then
  echo "--release-name, --namespace and --addon-name are required"
  exit 1
fi
echo "release: $release, namespace: $namespace, addon: $addon"

function takeOverResources() {
    local kind=$1
    local kind=$1
    crs=$(kubectl get $kind -l app.kubernetes.io/name=${addon} --no-headers)
    OLD_IFS=$IFS
    IFS=$'\n'
    for line in $crs; do
      name=$(echo "$line" | awk '{print $1}')
      kubectl annotate $kind $name --overwrite meta.helm.sh/release-name=$release
      kubectl annotate $kind $name --overwrite meta.helm.sh/release-namespace=$namespace
    done
    IFS=$OLD_IFS
}

takeOverResources "ClusterDefinition"
takeOverResources "ComponentVersion"