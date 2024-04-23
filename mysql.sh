mysql_cmd="mysql -u root -h ${DP_DB_HOST} -P${DP_DB_PORT} -p${DP_DB_PASSWORD} -N -e"
alterTableCount=`${mysql_cmd} "SELECT count(*) FROM INFORMATION_SCHEMA.INNODB_TABLES WHERE TOTAL_ROW_VERSIONS > 0;" | awk -F '\t' '{print}'`
if [ ${alterTableCount} -eq 0 ]; then
   echo "INFO: no tables need to optimize."
   exit 0
fi


leaderHost=`${mysql_cmd} "SELECT CURRENT_LEADER FROM INFORMATION_SCHEMA.WESQL_CLUSTER_LOCAL;"`
if [ -z ${leaderHost} ]; then
  echo "ERROR: no leader found in view INFORMATION_SCHEMA.WESQL_CLUSTER_LOCAL"
  exit 1
fi

# optimizer table in leader.
mysql_cmd="mysql -u root -h ${leaderHost} -P${DP_DB_PORT} -p${DP_DB_PASSWORD} -N -e"
OlD_IFS=${IFS}
${mysql_cmd} "SELECT name FROM INFORMATION_SCHEMA.INNODB_TABLES WHERE TOTAL_ROW_VERSIONS > 0;" | while IFS=$'\t' read -a row; do
 IFS=${OlD_IFS}
 table="${0////.}"
 echo "INFO: start to optimize table ${table}."
 ${mysql_cmd} "optimize table ${table};"
 echo "INFO: optimize table ${table} successfully."
done

