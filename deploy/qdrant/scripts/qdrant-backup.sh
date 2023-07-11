#!/usr/bin/env bash

set -e
mkdir -p ${BACKUP_DIR}
endpoint=http://${DB_HOST}:6333

snapshot=`curl -XPOST ${endpoint}/snapshots`
status=`echo ${snapshot} | jq '.status'`
if [ "${status}" != "ok" ]; then
    echo "backup failed, status: ${status}"
    exit 1
fi

name=`echo ${snapshot} | jq '.result.name'`
curl ${endpoint}/snapshots/${name} --output ${BACKUP_DIR}/${BACKUP_NAME}.snapshot

curl -XDELETE ${endpoint}/snapshots/${name}