#!/bin/bash
PARAM=$1
TYPE=$2
UPLOAD_REPO=$3

gh_curl() {
  curl -H "Accept: application/vnd.github.v3.raw" \
    $@
}

if [[ $TYPE == 1 ]]; then
  echo "${PARAM/v/}"
elif [[ $TYPE == 2 ]]; then
  GITHUB_API="https://api.github.com"
  gh_curl -s $GITHUB_API/repos/$UPLOAD_REPO/releases/tags/$PARAM > release_body.json
  echo $(jq '.upload_url' release_body.json) | sed 's/\"//g'
else
  echo "${PARAM/-/.}"
fi
