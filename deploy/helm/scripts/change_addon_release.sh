#!/bin/bash
release=$1
namespace=$2

function updateRelease() {
    local kind=$1
    crs=$(kubectl get $kind -l app.kubernetes.io/instance=${release} --no-headers)
    OLD_IFS=$IFS
    IFS=$'\n'
    for line in $crs; do
      name=$(echo "$line" | awk '{print $1}')
      kubectl annotate $kind $name --overwrite meta.helm.sh/release-name=$release
      kubectl annotate $kind $name --overwrite meta.helm.sh/release-namespace=$namespace
    done
    IFS=$OLD_IFS
}

updateRelease clusterdefinition
updateRelease componentversion

