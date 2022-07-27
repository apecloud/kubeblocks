#!/bin/bash

set -e
set -x

K3S_IMAGE_DIR=pkg/resources/static/k3s/images
K3S_IMAGE_NAME=k3s-airgap-images.tar.gz
K3S_OTHER_DIR=pkg/resources/static/k3s/other

TYPE=$1
ARCH=$2
K3S_VERSION=$3
GITHUB_PROXY=${GITHUB_PROXY:-https://github.91chi.fun/}

function download_k3s_images() {
   mkdir -p ${K3S_IMAGE_DIR}
   if [ -f "${K3S_IMAGE_DIR}/${K3S_IMAGE_NAME}" ]; then
   	echo "k3s image ${K3S_IMAGE_NAME} exists"
   else
   	curl -Lo ${K3S_IMAGE_NAME}/${K3S_IMAGE_NAME} ${GITHUB_PROXY}https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s-airgap-images-${ARCH}.tar.gz
   fi
}

function download_k3s_bin_script() {
  	mkdir -p ${K3S_OTHER_DIR}
  	curl -Lo ${K3S_OTHER_DIR}/k3s ${GITHUB_PROXY}https://github.com/k3s-io/k3s/releases/download/${K3S_VERSION}/k3s
  	curl -Lo ${K3S_OTHER_DIR}/setup.sh https://get.k3s.io
}

if [ "$TYPE" == "images" ]; then
  download_k3s_images
elif [ "$TYPE" == "other" ]; then
  download_k3s_bin_script
fi