#!/bin/bash

# export wal-g environments
backup_binlog_dir=${BACKUP_DIR}/${DP_TARGET_POD_NAME}
export WALG_MYSQL_DATASOURCE_NAME="${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DP_DB_PORT})/mysql"
export WALG_COMPRESSION_METHOD=zstd
export WALG_FILE_PREFIX=${backup_binlog_dir}
export WALG_MYSQL_CHECK_GTIDS=true
export MYSQL_PWD=${DB_PASSWORD}

# get binlog basename
MYSQL_CMD="mysql -u ${DB_USER} -h ${DB_HOST} -N"
log_bin_basename=$(${MYSQL_CMD} -e "SHOW VARIABLES LIKE 'log_bin_basename';" | awk -F'\t' '{print $2}')
if [ -z ${log_bin_basename} ]; then
   echo "ERROR: pod/${DP_TARGET_POD_NAME} connect failed."
   exit 1
fi
LOG_DIR=$(dirname $log_bin_basename)
LOG_PREFIX=$(basename $log_bin_basename)

latest_bin_log=""
last_flush_logs_time=$(date +%s)
last_purge_time=$(date +%s)
flush_bin_logs_interval=600

if [[ ${FLUSH_BINLOG_INTERVAL_SECONDS} =~ ^[0-9]+$ ]];then
  flush_bin_logs_interval=${FLUSH_BINLOG_INTERVAL_SECONDS}
fi

function log() {
    msg=$1
    local curr_date=$(date -u '+%Y-%m-%d %H:%M:%S')
    echo "${curr_date} INFO: $msg"
}

# checks if the mysql process is ok
function check_mysql_process() {
    is_ok=false
    for ((i=1;i<4;i++));do
      role=$(${MYSQL_CMD} -e "select role from information_schema.wesql_cluster_local;" | head -n 1)
      if [[ $? -eq 0  && (-z ${DP_TARGET_POD_ROLE} || "${DP_TARGET_POD_ROLE,,}" == "${role,,}") ]]; then
        is_ok=true
        break
      fi
      echo "Warning: target backup pod/${DP_TARGET_POD_NAME} is not OK, target role: ${DP_TARGET_POD_ROLE}, current role: ${role}, retry detection!"
      sleep 1
    done
    if [[ ${is_ok} == "false" ]];then
      echo "ERROR: target backup pod/${DP_TARGET_POD_NAME} is not OK, target role: ${DP_TARGET_POD_ROLE}, current role: ${role}!"
      exit 1
    fi
}

# clean up expired logfiles, interval is 60s
function purge_expired_files() {
  local curr_time=$(date +%s)
  local diff_time=$((${curr_time}-${last_purge_time}))
  if [[ -z ${LOGFILE_TTL_SECOND} || ${diff_time} -lt 60 ]]; then
     return
  fi
  if [[ -d ${backup_binlog_dir}/binlog_005 ]];then
     local retention_minute=$((${LOGFILE_TTL_SECOND}/60))
     local fileCount=$(find ${backup_binlog_dir}/binlog_005 -mmin +${retention_minute} -name "*.zst" | wc -l)
     find ${backup_binlog_dir}/binlog_005 -mmin +${retention_minute} -name "*.zst" -exec rm -rf {} \;
     if [ ${fileCount} -gt 0 ]; then
        log "clean up expired binlog file successfully, file count: ${fileCount}"
     fi
     last_purge_time=${curr_time}
  fi
}

# flush bin logs, interval is 600s by default
function flush_binlogs() {
  local curr_time=$(date +%s)
  local diff_time=$((${curr_time}-${last_flush_logs_time}))
  if [[ ${diff_time} -lt ${flush_bin_logs_interval} ]]; then
     return
  fi
  local LATEST_TRANS=$(mysqlbinlog $(ls -Ft $LOG_DIR/|grep -e '^mysql-bin.*[[:digit:]]$' |head -n 1)|grep 'Xid =' |head -n 1)
  # only flush bin logs when Xid exists
  if [[ -n "${LATEST_TRANS}" ]]; then
    log "flush binary logs"
    ${MYSQL_CMD} -e "flush binary logs";
  fi
  last_flush_logs_time=${curr_time}
}

# upload bin logs by wal-g
function upload_bin_logs() {
    latest_bin_log=$(ls -Ftr $LOG_DIR/|grep -e "^${LOG_PREFIX}.*[[:digit:]]$"|tail -n 1)
    wal-g binlog-push;
}

function get_binlog_start_time() {
  local binlog=$1
  local time=$(mysqlbinlog ${binlog} | grep -m 1 "end_log_pos" | awk '{print $1, $2}'|tr -d '#')
  local time=$(date -d "$time" -u '+%Y-%m-%dT%H:%M:%SZ')
  echo $time
}

function save_backup_status() {
  local first_bin_log=$(ls -Ftr $LOG_DIR/|grep -e "^${LOG_PREFIX}.*[[:digit:]]$"|head -n 1)
  local START_TIME=$(get_binlog_start_time $first_bin_log)
  local STOP_TIME=$(get_binlog_start_time $latest_bin_log)
  local TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
  if [[ -z $STOP_TIME ]];then
    echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
  else
    echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"startTime\":\"${START_TIME}\",\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
  fi
}

mkdir -p ${backup_binlog_dir} && cd $LOG_DIR
# trap term signal
trap "echo 'Terminating...' && sync && exit 0" TERM
log "start to archive binlog logs"
while true; do

  # check if mysql process is ok
  check_mysql_process

  # flush bin logs
  flush_binlogs

  # upload bin log
  upload_bin_logs

  # save backup status which will be updated to `backup` CR by the sidecar
  save_backup_status

  # purge the expired bin logs
  purge_expired_files
  sleep ${DP_INTERVAL_SECONDS}
done

