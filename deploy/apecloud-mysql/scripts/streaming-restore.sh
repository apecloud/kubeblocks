#! /bin/bash

if [ ! -f /etc/annotations/last-component-replicas ]; then
  exit 0
fi

component_replicas=`cat /etc/annotations/component-replicas`
last_component_replicas=`cat /etc/annotations/last-component-replicas`
if [ ${component_replicas} -ge ${last_component_replicas} ] || [ ${component_replicas} -eq 0 ]; then
  exit 0
fi

ordinal=${KB_POD_NAME##*-}
if [ ${ordinal} -lt ${component_replicas} ] || [ ${ordinal} -ge ${last_component_replicas} ]; then
  exit 0
fi

RESTORE_FILE=${DATA_VOLUME}/.xtrabackup_restore_done
if [ -f ${RESTORE_FILE} ]; then
  exit 0
fi

#while [ ! -f ${RESTORE_FILE} ];
#do
#    sleep 1
#done

RESTORE_TMP_DIR=${DATA_VOLUME}/restore-tmp
mkdir -p ${DATA_DIR} ${RESTORE_TMP_DIR}

cd ${RESTORE_TMP_DIR}
leader=${KB_LEADER}.${KB_CLUSTER_COMP_NAME}-headless
xbstream -x < nc -n ${leader} 3052

xtrabackup --decompress  --target-dir=${RESTORE_TMP_DIR}
xtrabackup --prepare --target-dir=${RESTORE_TMP_DIR}
find . -name "*.qp" | xargs rm -f
xtrabackup --move-back --target-dir=${RESTORE_TMP_DIR} --datadir=${DATA_DIR}/ --log-bin=${LOG_BIN}

cd ${DATA_VOLUME}
rm -rf ${RESTORE_TMP_DIR}
touch ${RESTORE_FILE}
touch ${DATA_DIR}/.xtrabackup_restore
chmod -R 0777 ${DATA_DIR}