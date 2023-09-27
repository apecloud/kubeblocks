set -e
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
mkdir -p ${DP_BACKUP_DIR} && cd ${DATA_DIR}
tar -czvf ${DP_BACKUP_DIR}/${DP_BACKUP_NAME}.tar.gz ./
echo "INFO: save data file successfully"
TOTAL_SIZE=$(du -shx ${DP_BACKUP_DIR}|awk '{print $1}')
echo "{\"totalSize\":\"$TOTAL_SIZE\"}" > ${DP_BACKUP_DIR}/backup.info && sync