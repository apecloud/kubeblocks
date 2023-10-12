#!/usr/bin/env bash

set -e
mkdir -p ${DP_BACKUP_DIR}
endpoint=http://${DP_DB_HOST}:6333

snapshot=`curl -XPOST ${endpoint}/snapshots`
status=`echo ${snapshot} | jq '.status'`
if [ "${status}" != "ok" ] && [ "${status}" != "\"ok\"" ]; then
    echo "backup failed, status: ${status}"
    exit 1
fi

name=`echo ${snapshot} | jq '.result.name'`
curl ${endpoint}/snapshots/${name} --output ${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.snapshot

curl -XDELETE ${endpoint}/snapshots/${name}

TOTAL_SIZE=$(du -shx ${DP_BACKUP_DIR}|awk '{print $1}')
echo "{\"totalSize\":\"$TOTAL_SIZE\"}" > ${DP_BACKUP_DIR}/backup.info