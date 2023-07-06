#!/usr/bin/env bash

set -x
set -o errexit
set -o errtrace
set -o nounset
set -o pipefail

curl=/qdrant/tools/curl
jq=/qdrant/tools/jq

idx=${KB_POD_NAME##*-}
current_component_replicas=$(cat /etc/annotations/component-replicas)
local_uri=http://localhost:6333

cluster_info=`$curl -s ${local_uri}/cluster`
local_peer_id=`echo "${cluster_info}"| $jq -r .result.peer_id`
leader_peer_id=`echo "${cluster_info}" | $jq -r .result.raft_info.leader`

move_shards() {
    cols=`$curl -s ${local_uri}/collections`
    col_count=`echo ${cols} | $jq -r '.result.collections | length'`
    if [[ ${col_count} -eq 0 ]]; then
        echo "no collections found in the cluster"
        return
    fi
    col_names=`echo ${cols} | $jq -r '.result.collections[].name'`
    for col_name in ${col_names}; do
        col_cluster_info=`$curl -s ${local_uri}/collections/${col_name}/cluster`
        col_shard_count=`echo ${col_cluster_info} | $jq -r '.result.local_shards[] | length'`
        if [[ ${col_shard_count} -eq 0 ]]; then
            echo "no shards found in collection ${col_name}"
            continue
        fi

        local_shard_ids=`echo ${col_cluster_info} | $jq -r '.result.local_shards[].shard_id'`
        for shard_id in ${local_shard_ids}; do
            echo "move shard ${shard_id} in col_name ${col_name} from ${local_peer_id} to ${leader_peer_id}"
            $curl -s -X POST -H "Content-Type: application/json" \
                -d '{"move_shard":{"shard_id": '${shard_id}',"to_peer_id": '${leader_peer_id}',"from_peer_id": '${local_peer_id}}}'' \
                ${local_uri}/collections/${col_name}/cluster
        done
    done
}

remove_peer() {
#    declare -A peer_to_uri=()
#    peer_ids="`echo ${cluster_info} | jq -r '.result.peers | keys'`"
#    for peer_id in "${peer_ids[@]}"; do
#        peer_uri=`echo ${cluster_info} | jq -r ".result.peers.${peer_id}.uri"`
#        peer_to_uri[peer_id]=peer_uri
#    done

    echo "remove local peer ${local_peer_id} from cluster"
    $curl -v -XDELETE ${local_uri}/cluster/peer/${local_peer_id}
}

if [ ! "$idx" -lt "$current_component_replicas" ] && [ "$current_component_replicas" -ne 0 ]; then
    echo "scaling in, we need to move local shards to other peers and remove local peer from the cluster"

    echo "cluster info: ${cluster_info}"

    move_shards

    remove_peer
else
    # stop, do nothing.
    echo "stop, do nothing"
fi