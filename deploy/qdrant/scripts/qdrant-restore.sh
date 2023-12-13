#!/usr/bin/env bash

set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

SNAPSHOT_DIR="${DATA_DIR}/_dp_snapshots"
mkdir -p "${SNAPSHOT_DIR}"
for snapshot in $(datasafed list /) ; do
  collection_name=${snapshot%.*}
  echo "INFO: start to restore collection ${collection_name}..."
  # download snapshot file
  datasafed pull "${snapshot}" "${SNAPSHOT_DIR}/${snapshot}"
  curl -X POST "http://${DP_DB_HOST}:6333/collections/${collection_name}/snapshots/upload?priority=snapshot" \
    -H 'Content-Type:multipart/form-data' \
    -F "snapshot=@${SNAPSHOT_DIR}/${snapshot}"
  echo "upload collection ${collection_name} successfully"
done


