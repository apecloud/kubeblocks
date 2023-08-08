function get_current_time() {
  curr_time=$(psql -U ${DB_USER} -h ${DB_HOST} -d postgres -t -c "SELECT now() AT TIME ZONE 'UTC'")
  echo $curr_time
}

function stat_and_save_backup_info() {
  START_TIME=$1
  STOP_TIME=$2
  if [ -z $STOP_TIME ]; then
    STOP_TIME=`get_current_time`
  fi
  START_TIME=$(date -d "${START_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
  STOP_TIME=$(date -d "${STOP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
  TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"startTime\":\"${START_TIME}\",\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
}