set -e
set -o pipefail
export PATH="$PATH:$DP_DATASAFED_BIN_PATH"
export DATASAFED_BACKEND_BASE_PATH="$DP_BACKUP_BASE_PATH"
connect_url="redis-cli -h ${DP_DB_HOST} -p ${DP_DB_PORT} -a ${DP_DB_PASSWORD}"
last_save=$(${connect_url} LASTSAVE)
echo "INFO: start BGSAVE"
${connect_url} BGSAVE
echo "INFO: wait for saving rdb successfully"
while true; do
  end_save=$(${connect_url} LASTSAVE)
  if [ $end_save -ne $last_save ];then
     break
  fi
  sleep 1
done
echo "INFO: start to save data file..."
cd ${DATA_DIR}
tar -czvf - ./ | datasafed push - "${DP_BACKUP_NAME}.tar.gz"
echo "INFO: save data file successfully"
TOTAL_SIZE=$(datasafed stat / | grep TotalSize | awk '{print $2}')
echo "{\"totalSize\":\"$TOTAL_SIZE\"}" > "${DP_BACKUP_INFO_FILE}" && sync
