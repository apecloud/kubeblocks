#!/bin/bash
set -e
if [ -d ${DP_BACKUP_DIR} ]; then
    rm -rf ${DP_BACKUP_DIR}
fi
mkdir -p ${DP_BACKUP_DIR}
START_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
xtrabackup --compress=zstd --backup --safe-slave-backup --slave-info --stream=xbstream \
    --host=${DP_DB_HOST} --user=${DP_DB_USER} --port=${DP_DB_PORT} --password=${DP_DB_PASSWORD} --datadir=${DATA_DIR} >${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.xbstream
STOP_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
TOTAL_SIZE=$(du -shx ${DP_BACKUP_DIR} | awk '{print $1}')
echo "{\"totalSize\":\"$TOTAL_SIZE\",\"timeRange\":{\"start\":\"${START_TIME}\",\"end\":\"${STOP_TIME}\"}}" >${DP_BACKUP_DIR}/backup.info
