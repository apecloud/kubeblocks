#!/usr/bin/env bash

set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
mkdir -p ${DATA_DIR}
res=`find ${DATA_DIR} -type f`
if [ ! -z "${res}" ]; then
  echo "${DATA_DIR} is not empty! Please make sure that the directory is empty before restoring the backup."
  exit 1
fi

# download snapshot file
SNAPSHOT_DIR="${DATA_DIR}/_dp_snapshots"
SNAPSHOT_FILE="${DP_BACKUP_NAME}.snapshot"
mkdir -p "${SNAPSHOT_DIR}"
datasafed pull "${SNAPSHOT_FILE}" "${SNAPSHOT_DIR}/${SNAPSHOT_FILE}"

# start qdrant restore process
/qdrant/qdrant --storage-snapshot "${SNAPSHOT_DIR}/${SNAPSHOT_FILE}"  --config-path /qdrant/config/config.yaml  --force-snapshot --uri http://localhost:6333 2>&1 | tee "${DATA_DIR}/restore.log" &

# wait until restore finished
# note: if we have curl, we can detect it this way
#   until curl http://localhost:6333/cluster; do sleep 1; done
until grep -q "Qdrant HTTP listening on" "${DATA_DIR}/restore.log"; do
  sleep 1
done
rm "${DATA_DIR}/restore.log"

# restore finished, we can kill the restore process now
pid=`pidof qdrant`
kill -s INT ${pid}
wait ${pid}

# delete snapshot file
rm -rf "${SNAPSHOT_DIR}"
