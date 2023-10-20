function get_current_time() {
  curr_time=$(psql -U ${DP_DB_USER} -h ${DP_DB_HOST} -d postgres -t -c "SELECT now() AT TIME ZONE 'UTC'")
  echo $curr_time
}

function stat_and_save_backup_info() {
  export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
  export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
  START_TIME=$1
  STOP_TIME=$2
  if [ -z $STOP_TIME ]; then
    STOP_TIME=`get_current_time`
  fi
  START_TIME=$(date -d "${START_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
  STOP_TIME=$(date -d "${STOP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
  TOTAL_SIZE=$(datasafed stat / | grep TotalSize | awk '{print $2}')
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"timeRange\":{\"start\":\"${START_TIME}\",\"end\":\"${STOP_TIME}\"}}" > "${DP_BACKUP_INFO_FILE}"
}