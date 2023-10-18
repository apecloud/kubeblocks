#!/usr/bin/env bash

set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

endpoint=http://${DP_DB_HOST}:6333

snapshot=`curl -XPOST ${endpoint}/snapshots`
status=`echo ${snapshot} | jq '.status'`
if [ "${status}" != "ok" ] && [ "${status}" != "\"ok\"" ]; then
    echo "backup failed, status: ${status}"
    exit 1
fi

name=`echo ${snapshot} | jq -r '.result.name'`
curl -v --fail-with-body ${endpoint}/snapshots/${name} | datasafed push - "/${DP_BACKUP_NAME}.snapshot"

curl -XDELETE ${endpoint}/snapshots/${name}

TOTAL_SIZE=$(datasafed stat / | grep TotalSize | awk '{print $2}')
echo "{\"totalSize\":\"$TOTAL_SIZE\"}" > "${DP_BACKUP_INFO_FILE}"