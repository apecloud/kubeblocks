mkdir -p ${BACKUP_DIR};
cd $LOG_DIR
LATEST_TRANS=$(mysqlbinlog $(ls -Ft $LOG_DIR/|grep -e '^mysql-bin.*[[:digit:]]$' |head -n 1)|grep 'Xid =' |head -n 1)
if [ -n "${LATEST_TRANS}" ]; then
  mysql -h ${DB_HOST} -P3306 -u ${DB_USER} -p${DB_PASSWORD} -e "flush binary logs";
fi
cd -

purge_expired_files() {
  if [[ ! -z ${LOGFILE_TTL_SECOND} && -d ${BACKUP_DIR}/binlog_005 ]];then
     retention_minute=$((${LOGFILE_TTL_SECOND}/60))
     fileCount=$(find ${BACKUP_DIR}/binlog_005 -mmin +${retention_minute} -name "*.zst" | wc -l)
     find ${BACKUP_DIR}/binlog_005 -mmin +${retention_minute} -name "*.zst" -exec rm -rf {} \;
     if [ ${fileCount} -gt 0 ]; then
        echo "clean up expired oplog file successfully, file count: ${fileCount}"
     fi
  fi
}
# purge expired files
purge_expired_files

latest_bin_log=$(ls -Ftr $LOG_DIR/|grep -e "^${LOG_PREFIX}.*[[:digit:]]$"|tail -n 1)
cat << EOF > $HOME/.walg.yaml
WALG_COMPRESSION_METHOD: zstd
WALG_MYSQL_DATASOURCE_NAME: ${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:3306)/mysql
#WALG_LOG_LEVEL: DEVEL
WALG_FILE_PREFIX: ${BACKUP_DIR}
WALG_MYSQL_CHECK_GTIDS: true
EOF
sync;
wal-g binlog-push;
echo "done."

function get_binlog_start_time() {
  binlog=$1
  time=$(mysqlbinlog ${binlog} | grep -m 1 "end_log_pos" | awk '{print $1, $2}'|tr -d '#')
  time=$(date -d "$time" -u '+%Y-%m-%dT%H:%M:%SZ')
  echo $time
}

function get_backup_time_range() {
    log_bin_basename=$(MYSQL_PWD=${DB_PASSWORD} mysql -u ${DB_USER} -h ${DB_HOST} -N -e "SHOW VARIABLES LIKE 'log_bin_basename';" | awk -F'\t' '{print $2}')
    LOG_DIR=$(dirname $log_bin_basename)
    LOG_PREFIX=$(basename $log_bin_basename)
    cd $LOG_DIR;
    first_bin_log=$(ls -Ftr $LOG_DIR/|grep -e "^${LOG_PREFIX}.*[[:digit:]]$"|head -n 1)
    START_TIME=$(get_binlog_start_time $first_bin_log)
    STOP_TIME=$(get_binlog_start_time $latest_bin_log)
    echo "$START_TIME,$STOP_TIME"
}

time_range=$(get_backup_time_range)
START_TIME=$(echo $time_range| awk -F "," '{print $1}')
STOP_TIME=$(echo $time_range| awk -F "," '{print $2}')
TOTAL_SIZE=$(du -shx ${BACKUP_DIR}|awk '{print $1}')
if [[ -z $STOP_TIME ]];then
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
else
  echo "{\"totalSize\":\"$TOTAL_SIZE\",\"manifests\":{\"backupLog\":{\"startTime\":\"${START_TIME}\",\"stopTime\":\"${STOP_TIME}\"},\"backupTool\":{\"uploadTotalSize\":\"${TOTAL_SIZE}\"}}}" > ${BACKUP_DIR}/backup.info
fi
