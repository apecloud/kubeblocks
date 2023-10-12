#!/bin/bash
set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
START_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
xtrabackup --compress=zstd --backup --safe-slave-backup --slave-info --stream=xbstream \
    --host=${DP_DB_HOST} --port=${DP_DB_PORT} \
    --user=${DP_DB_USER} --password=${DP_DB_PASSWORD} \
    --datadir=${DATA_DIR} | datasafed push - "/${DP_BACKUP_NAME}.xbstream"
STOP_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
TOTAL_SIZE=$(datasafed stat / | grep TotalSize | awk '{print $2}')
echo "{\"totalSize\":\"$TOTAL_SIZE\",\"timeRange\":{\"start\":\"${START_TIME}\",\"end\":\"${STOP_TIME}\"}}" > "${DP_BACKUP_INFO_FILE}"
