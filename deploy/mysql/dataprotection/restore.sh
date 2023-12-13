#!/bin/bash
set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
mkdir -p ${DATA_DIR}
TMP_DIR=/data/mysql/temp
mkdir -p ${TMP_DIR} && cd ${TMP_DIR}
datasafed pull "${DP_BACKUP_NAME}.xbstream" - | xbstream -x
xtrabackup --decompress --remove-original --target-dir=${TMP_DIR}
xtrabackup --prepare --target-dir=${TMP_DIR}
xtrabackup --move-back --target-dir=${TMP_DIR} --datadir=${DATA_DIR}/
rm -rf ${TMP_DIR}
chmod -R 0777 ${DATA_DIR}
