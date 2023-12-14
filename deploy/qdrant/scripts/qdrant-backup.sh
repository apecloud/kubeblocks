#!/usr/bin/env bash

set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

# if the script exits with a non-zero exit code, touch a file to indicate that the backup failed,
# the sync progress container will check this file and exit if it exists
function handle_exit() {
  exit_code=$?
  if [ $exit_code -ne 0 ]; then
    echo "failed with exit code $exit_code"
    touch "${DP_BACKUP_INFO_FILE}.exit"
    exit 1
  fi
}
trap handle_exit EXIT

endpoint=http://${DP_DB_HOST}:6333
collectionRes=$(curl ${endpoint}/collections)
collections=$(echo ${collectionRes}  | jq -r '.result.collections[].name')
# snapshot all collections
for c in ${collections}; do
  echo "INFO: start to snapshot collection ${c}..."
  snapshot=$(curl -XPOST ${endpoint}/collections/${c}/snapshots)
  status=$(echo ${snapshot} | jq '.status')
  if [ "${status}" != "ok" ] && [ "${status}" != "\"ok\"" ]; then
    echo "backup failed, status: ${status}"
    exit 1
  fi
  name=$(echo ${snapshot} | jq -r '.result.name')
  curl -v --fail-with-body ${endpoint}/collections/${c}/snapshots/${name} | datasafed push - "/${c}.snapshot"
  curl -XDELETE ${endpoint}/collections/${c}/snapshots/${name}
  echo "INFO: snapshot collection ${c} successfully."
done
TOTAL_SIZE=$(datasafed stat / | grep TotalSize | awk '{print $2}')
echo "{\"totalSize\":\"$TOTAL_SIZE\"}" >"${DP_BACKUP_INFO_FILE}"
