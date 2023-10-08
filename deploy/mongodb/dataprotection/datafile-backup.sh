if [ -d ${DP_BACKUP_DIR} ]; then
  rm -rf ${DP_BACKUP_DIR}
fi
mkdir -p ${DP_BACKUP_DIR} && cd ${DATA_DIR}
START_TIME=`get_current_time`
# TODO: flush data and locked write, otherwise data maybe inconsistent
tar -czvf ${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.tar.gz ./
rm -rf mongodb.backup
# stat and save the backup information
stat_and_save_backup_info $START_TIME