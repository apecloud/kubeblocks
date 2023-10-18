set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
cd ${DATA_DIR}
START_TIME=`get_current_time`
# TODO: flush data and locked write, otherwise data maybe inconsistent
tar -czvf - ./ | datasafed push - "${DP_BACKUP_NAME}.tar.gz"
rm -rf mongodb.backup
# stat and save the backup information
stat_and_save_backup_info $START_TIME