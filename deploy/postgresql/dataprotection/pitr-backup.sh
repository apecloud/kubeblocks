#!/bin/bash
export PGPASSWORD=${DB_PASSWORD}
PSQL="psql -h ${DB_HOST} -U ${DB_USER} -d postgres"
last_switch_wal_time=$(date +%s)
last_purge_time=$(date +%s)
STOP_TIME=
switch_wal_interval=300

if [[ ${SWITCH_WAL_INTERVAL_SECONDS} =~ ^[0-9]+$ ]];then
  switch_wal_interval=${SWITCH_WAL_INTERVAL_SECONDS}
fi

backup_in_secondary=
if [ "${DP_POD_ROLE}" == "primary" ]; then
   backup_in_secondary=f
elif [ "${DP_POD_ROLE}" == "secondary" ]; then
   backup_in_secondary=t
fi

function log() {
    msg=$1
    local curr_date=$(date -u '+%Y-%m-%d %H:%M:%S')
    echo "${curr_date} INFO: $msg"
}

function purge_expired_files() {
    # clean up expired logfiles, interval is 60s
    local curr_time=$(date +%s)
    local diff_time=$((${curr_time}-${last_purge_time}))
    if [[ -z ${LOGFILE_TTL_SECOND} || ${diff_time} -lt 60 ]]; then
       return
    fi
    retention_day=$((${LOGFILE_TTL_SECOND}/86400))
    EXPIRED_INCR_LOG=${BACKUP_DIR}/$(date -d"${retention_day} day ago" +%Y%m%d);
    if [ -d ${EXPIRED_INCR_LOG} ]; then
      rm -rf ${EXPIRED_INCR_LOG};
    fi
    last_purge_time=${curr_time}
}

function switch_wal_log() {
    local curr_time=$(date +%s)
    local diff_time=$((${curr_time}-${last_switch_wal_time}))
    if [[ ${diff_time} -lt ${switch_wal_interval} ]]; then
       return
    fi
    LAST_TRANS=$(pg_waldump $(${PSQL} -Atc "select pg_walfile_name(pg_current_wal_lsn())") --rmgr=Transaction 2>/dev/null |tail -n 1)
    if [ "${LAST_TRANS}" != "" ] && [ "$(find ${LOG_DIR}/archive_status/ -name '*.ready')" = "" ]; then
      log "start to switch wal file"
      ${PSQL} -c "select pg_switch_wal()"
      for i in $(seq 1 60); do
        if [ "$(find ${LOG_DIR}/archive_status/ -name '*.ready')" != "" ]; then
          log "switch wal file successfully"
          break;
        fi
        sleep 1
      done
    fi
    last_switch_wal_time=${curr_time}
}

function upload_wal_log() {
    TODAY_INCR_LOG=${BACKUP_DIR}/$(date +%Y%m%d);
    mkdir -p ${TODAY_INCR_LOG};
    cd ${LOG_DIR}
    for i in $(ls -tr ./archive_status/ | grep .ready); do
      wal_name=${i%.*}
      LOG_STOP_TIME=$(pg_waldump ${wal_name} --rmgr=Transaction 2>/dev/null |tail -n 1|awk -F ' COMMIT ' '{print $2}'|awk -F ';' '{print $1}')
      if [[ ! -z $LOG_STOP_TIME ]];then
        STOP_TIME=$(date -d "${LOG_STOP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
      fi
      if [ -f ${wal_name} ]; then
        log "upload ${wal_name}"
        gzip -kqc ${wal_name} > ${TODAY_INCR_LOG}/${wal_name}.gz;
        mv -f ./archive_status/${i} ./archive_status/${wal_name}.done;
      fi
    done
}

function save_backup_status() {
    TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
    if [[ -z ${STOP_TIME} ]];then
      echo "{\"totalSize\":\"${TOTAL_SIZE}\",\"manifests\":{\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
    else
      echo "{\"totalSize\":\"${TOTAL_SIZE}\",\"manifests\":{\"backupLog\":{\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
    fi
}

function check_pg_process() {
    is_ok=false
    for ((i=1;i<4;i++));do
      is_secondary=$(${PSQL} -Atc "select pg_is_in_recovery()")
      if [[ $? -eq 0  && (-z ${backup_in_secondary} || "${backup_in_secondary}" == "${is_secondary}") ]]; then
        is_ok=true
        break
      fi
      echo "Warning: target backup pod/${DP_TARGET_POD_NAME} is not OK, target role: ${DP_POD_ROLE}, pg_is_in_recovery: ${is_secondary}, retry detection!"
      sleep 1
    done
    if [[ ${is_ok} == "false" ]];then
      echo "ERROR: target backup pod/${DP_TARGET_POD_NAME} is not OK, target role: ${DP_POD_ROLE}, pg_is_in_recovery: ${is_secondary}!"
      exit 1
    fi
}

# trap term signal
trap "echo 'Terminating...' && sync && exit 0" TERM
log "start to archive wal logs"
while true; do

  # check if pg process is ok
  check_pg_process

  # switch wal log
  switch_wal_log

  # upload wal log
  upload_wal_log

  # save backup status which will be updated to `backup` CR by the sidecar
  save_backup_status

  # purge the expired wal logs
  purge_expired_files
  sleep ${DP_INTERVAL_SECONDS}
done