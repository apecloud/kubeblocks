set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"

mongo_uri="mongodb://${DP_DB_HOST}:${DP_DB_PORT}"
datasafed pull "${DP_BACKUP_NAME}.archive" - | mongorestore --archive --gzip --uri "${mongo_uri}" -u ${MONGODB_ROOT_USER} -p ${MONGODB_ROOT_PASSWORD} --authenticationDatabase admin
