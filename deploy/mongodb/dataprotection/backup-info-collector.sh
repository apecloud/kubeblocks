function get_current_time() {
  CLIENT=`which mongosh&&echo mongosh||echo mongo`
  curr_time=$(${CLIENT} -u ${DB_USER} -p ${DB_PASSWORD} --port 27017 --host ${DB_HOST} --authenticationDatabase admin  --eval 'db.isMaster().lastWrite.lastWriteDate.getTime()/1000' --quiet)
  curr_time=$(date -d "@${curr_time}" -u '+%Y-%m-%dT%H:%M:%SZ')
  echo $curr_time
}

function stat_and_save_backup_info() {
  START_TIME=$1
  STOP_TIME=$2
  if [ -z $STOP_TIME ]; then
    STOP_TIME=`get_current_time`
  fi
  TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"startTime\":\"${START_TIME}\",\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
}