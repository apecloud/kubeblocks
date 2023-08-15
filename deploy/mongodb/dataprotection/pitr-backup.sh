#!/bin/bash
mkdir -p ${BACKUP_DIR} && cd ${BACKUP_DIR}
# retention 8 days by default
retention_minute=""
if [ ! -z ${LOGFILE_TTL_SECOND} ];then
  retention_minute=$((${LOGFILE_TTL_SECOND}/60))
fi
export MONGODB_URI="mongodb://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:27017/?authSource=admin"
export WALG_FILE_PREFIX=${BACKUP_DIR}
export OPLOG_ARCHIVE_TIMEOUT_INTERVAL=${ARCHIVE_INTERVAL}
export OPLOG_ARCHIVE_AFTER_SIZE=${ARCHIVE_AFTER_SIZE}
retryTimes=0
purgeCounter=0
wal_g_pid=0

do_oplog_push(){
  echo "start to archive oplog..."
  echo "wal-g oplog-push > /tmp/wal-g-oplog.log"
  wal-g oplog-push >/tmp/wal-g-oplog.log 2>&1 &
  wal_g_pid=$!
  sleep 1
  cat /tmp/wal-g-oplog.log
}

check_oplog_push_process(){
  # check wal-g oplog-push process
  ps -p $wal_g_pid >/dev/null
  if [ $? -ne 0 ]; then
    echo 'ERROR: the process "wal-g oplog-push" does not exist!'
    errorLog=$(cat /tmp/wal-g-oplog.log)
    echo $errorLog && exit 1
  fi
  # check role of the connected mongodb
  isPrimary=$(mongosh -u ${DB_USER} -p ${DB_PASSWORD} --port 27017 --host ${DB_HOST} --authenticationDatabase admin  --eval 'db.isMaster().ismaster' --quiet)
  if [ "${isPrimary}" != "true" ]; then
    echo "isPrimary: ${isPrimary}"
    retryTimes=$(($retryTimes+1))
  else
    retryTimes=0
  fi
  if [ $retryTimes -ge 3 ]; then
     echo "ERROR: the current mongo instance is not a primary node, 3 attempts have been made!" && kill $wal_g_pid
  fi
}

save_backup_status() {
   TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
   OLDEST_FILE=$(ls -t ${BACKUP_DIR}/oplog_005 | tail -n 1) && OLDEST_FILE=${OLDEST_FILE#*_} && LOG_START_TIME=${OLDEST_FILE%%.*}
   LATEST_FILE=$(ls -t ${BACKUP_DIR}/oplog_005 | head -n 1) && LATEST_FILE=${LATEST_FILE##*_} && LOG_STOP_TIME=${LATEST_FILE%%.*}
   if [ ! -z $LOG_START_TIME ]; then
       START_TIME=$(date -d "@${LOG_START_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
       STOP_TIME=$(date -d "@${LOG_STOP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
       echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"startTime\":\"${START_TIME}\",\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
   fi
}
# purge the expired files
purge_expired_files() {
  if [ ! -z ${LOGFILE_TTL_SECOND} ];then
    purgeCounter=$((purgeCounter+3))
    if [ $purgeCounter -ge 60 ]; then
       purgeCounter=0
       fileCount=$(find ${BACKUP_DIR}/oplog_005 -mmin +${retention_minute} -name "*.lz4" | wc -l)
       find ${BACKUP_DIR}/oplog_005 -mmin +${retention_minute} -name "*.lz4" -exec rm -rf {} \;
       if [ ${fileCount} -gt 0 ]; then
          echo "clean up expired oplog file successfully, file count: ${fileCount}"
       fi
    fi
  fi
}
# create oplog push process
do_oplog_push
# trap term signal
trap "echo 'Terminating...' && kill $wal_g_pid" TERM
while true; do
  check_oplog_push_process
  sleep 1
  if [ -d ${BACKUP_DIR}/oplog_005 ];then
    save_backup_status
    # purge the expired oplog
    purge_expired_files
  fi
done