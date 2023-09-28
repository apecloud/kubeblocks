#!/bin/bash
set -e
mkdir -p ${DATA_DIR}
TMP_DIR=${DATA_MOUNT_DIR}/temp
mkdir -p ${TMP_DIR} && cd ${TMP_DIR}
xbstream -x <${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.xbstream
xtrabackup --decompress --remove-original --target-dir=${TMP_DIR}
xtrabackup --prepare --target-dir=${TMP_DIR}
xtrabackup --move-back --target-dir=${TMP_DIR} --datadir=${DATA_DIR}/ --log-bin=${LOG_BIN}
touch ${DATA_DIR}/${SIGNAL_FILE}
rm -rf ${TMP_DIR}
chmod -R 0777 ${DATA_DIR}
