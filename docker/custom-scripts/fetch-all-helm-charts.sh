#!/usr/bin/env bash
#
# This script will fetch all dependent helm charts.
#
# Syntax: ./fetch-all-helm-charts.sh KB_CHART_DIR TARGET_DIR

set -e

if [ $# -ne 2 ]; then
  echo "Syntax: ./fetch-all-helm-charts.sh KB_CHART_DIR TARGET_DIR"
  exit 1
fi

KB_CHART_DIR=${1}
TARGET_DIR=${2:-"charts"}
MANIFESTS_DIR="/tmp/manifests/"

ADDON_DIR="kubeblocks/templates/addons"
APP_DIR="kubeblocks/templates/applications"
ADDON_HELM_CHART_URL=https://jihulab.com/api/v4/projects/150246/packages/helm/stable/charts
APP_HELM_CHART_URL=https://jihulab.com/api/v4/projects/152630/packages/helm/stable/charts

# fetch helm charts to target directory
# parameters:
#   $1: helm repo url
#   $2: addon CRs directory
fetch_helm_charts() {
  helm template "${KB_CHART_DIR}" --output-dir "${MANIFESTS_DIR}" --set addonChartLocationBase="$1"
  # travel all addon manifests and get the helm charts
  for f in "${MANIFESTS_DIR}$2"/*; do
    if [ -d "${f}" ]; then
      continue
    fi

    kind=$(yq eval '.kind' "${f}")
    if [ "${kind}" != "Addon" ]; then
      continue
    fi

    # get helm chart location
    chartURL=$(yq eval '.spec.helm.chartLocationURL' "${f}")
    if [ -z "${chartURL}" ]; then
      echo "chartLocationURL is empty in ${f}"
      exit 1
    fi

    # fetch the helm chart
    echo "fetching helm chart from ${chartURL}"
    helm fetch "$chartURL" -d "${TARGET_DIR}"
  done
}

# make directories
mkdir -p "${TARGET_DIR}"
mkdir -p "${MANIFESTS_DIR}"

# get all manifests
helm version

echo "fetch addons helm charts, addon CRs directory: ${ADDON_DIR}, helm chart url: ${ADDON_HELM_CHART_URL}"
fetch_helm_charts "${ADDON_HELM_CHART_URL}" "${ADDON_DIR}"

echo "fetch applications helm charts, applications CRs directory: ${APP_DIR}, helm chart url: ${APP_HELM_CHART_URL}"
fetch_helm_charts "${APP_HELM_CHART_URL}" "${APP_DIR}"