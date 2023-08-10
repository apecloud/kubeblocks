#!/bin/bash
set -e;
# clean up expired logfiles
if [ ! -z ${LOGFILE_TTL_SECOND} ];then
  retention_day=$((${LOGFILE_TTL_SECOND}/86400))
  EXPIRED_INCR_LOG=${BACKUP_DIR}/$(date -d"${retention_day} day ago" +%Y%m%d);
  if [ -d ${EXPIRED_INCR_LOG} ]; then
    rm -rf ${EXPIRED_INCR_LOG};
  fi
fi

export PGPASSWORD=${DB_PASSWORD}
PSQL="psql -h ${DB_HOST} -U ${DB_USER} -d postgres"
LAST_TRANS=$(pg_waldump $(${PSQL} -Atc "select pg_walfile_name(pg_current_wal_lsn())") --rmgr=Transaction 2>/dev/null |tail -n 1)
if [ "${LAST_TRANS}" != "" ] && [ "$(find ${LOG_DIR}/archive_status/ -name '*.ready')" = "" ]; then
  echo "switch wal file"
  ${PSQL} -c "select pg_switch_wal()"
  for i in $(seq 1 60); do
    echo "waiting wal ready ..."
    if [ "$(find ${LOG_DIR}/archive_status/ -name '*.ready')" != "" ]; then break; fi
    sleep 1
  done
fi

STOP_TIME=""
TODAY_INCR_LOG=${BACKUP_DIR}/$(date +%Y%m%d);
mkdir -p ${TODAY_INCR_LOG};
cd ${LOG_DIR}
for i in $(ls -tr ./archive_status/*.ready); do
  wal_ready_name="${i##*/}"
  wal_name=${wal_ready_name%.*}
  echo "uploading ${wal_name}";
  LOG_STOP_TIME=$(pg_waldump ${wal_name} --rmgr=Transaction 2>/dev/null |tail -n 1|awk -F ' COMMIT ' '{print $2}'|awk -F ';' '{print $1}')
  if [[ ! -z $LOG_STOP_TIME ]];then
    STOP_TIME=$(date -d "${LOG_STOP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
  fi
  gzip -kqc ${wal_name} > ${TODAY_INCR_LOG}/${wal_name}.gz;
  mv -f ${i} ./archive_status/${wal_name}.done;
done
echo "done."
sync;
TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
if [[ -z $STOP_TIME ]];then
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
else
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
fi