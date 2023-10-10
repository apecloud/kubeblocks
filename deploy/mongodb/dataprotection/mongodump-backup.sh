if [ -d ${DP_BACKUP_DIR} ]; then
  rm -rf ${DP_BACKUP_DIR}
fi
mkdir -p ${DP_BACKUP_DIR}

# TODO: support endpoint env for sharding cluster.
mongo_uri="mongodb://${DP_DB_HOST}:${DP_DB_PORT}"
START_TIME=`get_current_time`
mongodump --uri ${mongo_uri} -u ${DP_DB_USER} -p ${DP_DB_PASSWORD} --authenticationDatabase admin --out ${DP_BACKUP_DIR}

# stat and save the backup information
stat_and_save_backup_info $START_TIME