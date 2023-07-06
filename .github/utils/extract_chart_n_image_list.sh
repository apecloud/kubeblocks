#!/bin/bash

KB_VERSION=${1:-0.5.2}
KB_HELM_REPO_INDEX_URL_BASE=https://jihulab.com/api/v4/projects/85949/packages/helm/stable
KB_HELM_REPO_INDEX_URL=${KB_HELM_REPO_INDEX_URL_BASE}/index.yaml

# set -o errexit
# set -o nounset
# set -o pipefail

print_error() {
  echo "$1" >&2
}

# List of required commands
required_cmds=("curl" "helm" "jq" "yq")

# Loop through the list of commands and check if they exist
for cmd in "${required_cmds[@]}"; do
    if ! command -v "$cmd" &> /dev/null; then
        print_error "Error: '$cmd' command not found"
        exit 1
    fi
done

# Regular expression to match http or https
regex="^(http|https)://.*"

kb_index_json=`curl ${KB_HELM_REPO_INDEX_URL} | yq eval -ojson`
entries=`echo ${kb_index_json} | jq -r '.entries | keys | .[]'`

chart_url_array=()
image_array=()
for entry in ${entries}
do
    version=${KB_VERSION}
    url=""
    helm_custom_args=""
    images=""

    # specialized processor
    case ${entry} in
        # ignored entries
        "agamotto" | "apecloud-mysql-scale" | "apecloud-mysql-scale-cluster" | "chaos-mesh" | "chatgpt-retrieval-plugin" | "clickhouse" | "delphic" | "etcd" | "etcd-cluster" | "kafka" | "opensearch" | "opensearch-cluster" | "redis-demo" | "prometheus-kubeblocks" )
            continue
            ;;
        "aws-load-balancer-controller")
            helm_custom_args="--set clusterName=clusterName"
            ;;
        "dt-platform" | "kubeblocks-csi-driver")
            # following chart is missing from chart repo index
            version="0.1.0"
            # url=https://jihulab.com/api/v4/projects/85949/packages/helm/stable/charts/${entry}-${version}.tgz
            ;;
        "prometheus")
            version="15.16.1"
            ;;
    esac

    # compose helm chart URL 
    if [ -z "${url}" ]; then
        select_entry=`echo ${kb_index_json} | jq -r ".entries[\"${entry}\"][] | select(.version == \"${version}\")"`
        url=`echo ${select_entry} | jq -r '.urls[0]'`
        if [ -z "$url" ]; then
            # choose lastest version instead
            select_entry=`echo ${kb_index_json} | jq -r ".entries[\"${entry}\"][0]"`
            url=`echo ${select_entry} | jq -r '.urls[0]'`
            version=`echo ${select_entry} | jq -r '.version'`
        fi
        if ! [[ $url =~ $regex ]]; then
            url=${KB_HELM_REPO_INDEX_URL_BASE}/${url}
        fi
    fi

    # extract images from helm templates
    if [ -z "${images}" ]; then
        images=`helm template ${entry} ${url} ${helm_custom_args} | grep "image:" | awk '{print $2}'`
    fi

    chart_url_array+=(${url})
    print_error "processed entry=${entry}; version=${version}; url=${url}"
    for image in ${images}
    do
        image="${image//\"}"
        image_array+=(${image})
    done
done

print_error "" 

# Convert array to set
image_set=($(printf "%s\n" "${image_array[@]}" | sort -u))

# Convert to JSON
chart_url_json_arr="[$(printf '"%s",' "${chart_url_array[@]}" | sed 's/,$//')]"
images_json_arr="[$(printf '"%s",' "${image_set[@]}" | sed 's/,$//')]"
json_out="{\"chartURLs\":${chart_url_json_arr},\"images\":${images_json_arr}}"
echo $json_out | jq -r '.'
