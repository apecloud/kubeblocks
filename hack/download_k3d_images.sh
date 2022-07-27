#!/bin/bash

set -e
set -x

K3D_IMAGE_DIR=pkg/resources/static/k3d/images
ARCH=$1
K3S_VERSION=${2:-v1.23.8-k3s1}
K3D_VERSION=${3:-5.4.4}

mkdir -p "$K3D_IMAGE_DIR"

function download_k3d_images() {
  k3d_images=(
    "ghcr.io/k3d-io/k3d-tools:${K3D_VERSION}"
    "ghcr.io/k3d-io/k3d-proxy:${K3D_VERSION}"
    "docker.io/rancher/k3s:${K3S_VERSION}")

  for IMG in ${k3d_images[*]}; do
    IMAGE_NAME=$(echo "$IMG" | cut -f1 -d: | cut -f3 -d/)

    echo saving "$IMG" to "$K3D_IMAGE_DIR"/"$IMAGE_NAME".tar
    if [ -f "$K3D_IMAGE_DIR"/"$IMAGE_NAME".tar.gz ]; then
      echo "$K3D_IMAGE_DIR/$IMAGE_NAME.tar.gz exists"
      continue
    fi

    $DOCKER_PULL "$IMG"
    docker save -o "$K3D_IMAGE_DIR"/"$IMAGE_NAME".tar "$IMG"
    gzip -f "$K3D_IMAGE_DIR"/"$IMAGE_NAME".tar
  done
}

function determine_pull_command() {
  DOCKER_PULL="docker pull --platform=linux/amd64"
  if [ "$1" == "arm64" ]; then
      DOCKER_PULL="docker pull --platform=linux/arm64"
  fi
}

determine_pull_command "$ARCH"
download_k3d_images