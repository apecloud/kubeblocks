set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

# TODO: support endpoint env for sharding cluster.
mongo_uri="mongodb://${DP_DB_HOST}:${DP_DB_PORT}"
START_TIME=`get_current_time`
mongodump --uri "${mongo_uri}" -u ${DP_DB_USER} -p ${DP_DB_PASSWORD} --authenticationDatabase admin --archive --gzip | datasafed push - "${DP_BACKUP_NAME}.archive"

# stat and save the backup information
stat_and_save_backup_info $START_TIME