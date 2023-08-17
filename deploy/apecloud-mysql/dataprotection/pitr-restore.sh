if [ -e ${PITR_DIR}/replay.sql ]; then
  echo "replay SQL '${PITR_DIR}/replay.sql' file exists, skip restore.";
  exit 0;
fi
mkdir -p ${PITR_DIR};
cat << EOF > $HOME/.walg.yaml
WALG_COMPRESSION_METHOD: zstd
#WALG_LOG_LEVEL: DEVEL
WALG_FILE_PREFIX: ${BACKUP_DIR}
WALG_MYSQL_CHECK_GTIDS: true
WALG_MYSQL_BINLOG_DST: ${PITR_DIR}
WALG_MYSQL_BINLOG_REPLAY_COMMAND: mysqlbinlog --stop-datetime="\$WALG_MYSQL_BINLOG_END_TS" "\$WALG_MYSQL_CURRENT_BINLOG" >> ${PITR_DIR}/replay.sql
EOF
sync;
BASE_BACKUP_TIME=${BASE_BACKUP_START_TIME}
if [ -f $DATA_DIR/xtrabackup_info ]; then
  BASE_BACKUP_TIME=$(cat $DATA_DIR/xtrabackup_info | grep start_time | awk -F ' = ' '{print $2}');
  BASE_BACKUP_TIME=$(date -d"${BASE_BACKUP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
fi
BINLOG_LATEST_TIME=$(date -d"${KB_RECOVERY_TIME} +1 day" -u '+%Y-%m-%dT%H:%M:%SZ')
wal-g binlog-replay --since-time "${BASE_BACKUP_TIME}" --until "${KB_RECOVERY_TIME}" --until-binlog-last-modified-time "${BINLOG_LATEST_TIME}";
echo "done.";
sync;