#!/bin/sh
# usage: xstore-post-start.sh type_name
# type_name: component.type, in uppercase.

TYPE_NAME=$1

# setup shared-channel.json
SHARED_CHANNEL_JSON='{"nodes": ['

i=0
while [ $i -lt $(eval echo \$KB_"$TYPE_NAME"_N) ]; do
  hostname=$(eval echo \$KB_"$TYPE_NAME"_"$i"_HOSTNAME)
  pod=$(echo "$hostname" | cut -d'.' -f1)

  NODE_OBJECT=$(printf '{"pod": "%s", "host": "%s", "port": 11306, "role": "candidate", "node_name": "%s" }' "$pod" "$hostname" "$pod")
  SHARED_CHANNEL_JSON+="$NODE_OBJECT,"
  i=$(( i + 1))
done

SHARED_CHANNEL_JSON=${SHARED_CHANNEL_JSON%,}
SHARED_CHANNEL_JSON+=']}'

mkdir -p /data/shared/
echo $SHARED_CHANNEL_JSON > /data/shared/shared-channel.json
