mongo_uri="mongodb://${DP_DB_HOST}:${DP_DB_PORT}"
for dir_name in $(ls ${DP_BACKUP_DIR} -l | grep ^d | awk '{print $9}'); do
  database_dir=${DP_BACKUP_DIR}/$dir_name
  echo "INFO: restoring from ${database_dir}"
  mongorestore --uri ${mongo_uri} -u ${MONGODB_ROOT_USER} -p ${MONGODB_ROOT_PASSWORD} -d $dir_name --authenticationDatabase admin ${database_dir}
done