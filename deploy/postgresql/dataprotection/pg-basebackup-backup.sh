set -e;
if [ -d ${DP_BACKUP_DIR} ]; then
  rm -rf ${DP_BACKUP_DIR}
fi
mkdir -p ${DP_BACKUP_DIR};
export PGPASSWORD=${DP_DB_PASSWORD}

START_TIME=`get_current_time`
echo ${DP_DB_PASSWORD} | pg_basebackup -Ft -Pv -c fast -Xs -Z${COMPRESS_LEVEL} -D ${DP_BACKUP_DIR} -h ${DP_DB_HOST} -U ${DP_DB_USER} -W;

# stat and save the backup information
stat_and_save_backup_info $START_TIME