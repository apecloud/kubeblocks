#!/bin/bash
while [ "$KB_PROXY_ENABLED" != "on" ]
do
  sleep 60
done

. /scripts/set_config_variables.sh
set_config_variables vttablet

cell=${CELL:-'zone1'}
uid="${KB_POD_NAME##*-}"
mysql_root=${MYSQL_ROOT_USER:-'root'}
mysql_root_passwd=${MYSQL_ROOT_PASSWORD:-'123456'}
mysql_port=${MYSQL_PORT:-'3306'}
port=${VTTABLET_PORT:-'15100'}
grpc_port=${VTTABLET_GRPC_PORT:-'16100'}
vtctld_host=${VTCTLD_HOST:-'127.0.0.1'}
vtctld_web_port=${VTCTLD_WEB_PORT:-'15000'}
printf -v alias '%s-%010d' $cell $uid
printf -v tablet_dir 'vt_%010d' $uid
tablet_hostname=$(eval echo \$KB_"$uid"_HOSTNAME)
printf -v tablet_logfile 'vttablet_%010d_querylog.txt' $uid

tablet_type=replica
topology_fags=${TOPOLOGY_FLAGS:-'--topo_implementation etcd2 --topo_global_server_address 127.0.0.1:2379 --topo_global_root /vitess/global'}

/scripts/wait-for-service.sh vtctld $vtctld_host $vtctld_web_port

echo "starting vttablet for $alias..."

VTDATAROOT=$VTDATAROOT/vttablet
su vitess <<EOF
mkdir -p $VTDATAROOT
exec vttablet \
$topology_fags \
--alsologtostderr \
$(if [ "$enable_logs" == "true" ]; then echo "--log_dir $VTDATAROOT"; fi) \
$(if [ "$enable_query_log" == "true" ]; then echo "--log_queries_to_file $VTDATAROOT/$tablet_logfile"; fi) \
--tablet-path $alias \
--tablet_hostname "$tablet_hostname" \
--init_tablet_type $tablet_type \
--health_check_interval $health_check_interval \
--shard_sync_retry_delay $shard_sync_retry_delay \
--remote_operation_timeout $remote_operation_timeout \
--db_connect_timeout_ms $db_connect_timeout_ms \
--enable_replication_reporter \
--backup_storage_implementation file \
--file_backup_storage_root $VTDATAROOT/backups \
--port $port \
--db_port $mysql_port \
--db_host $tablet_hostname \
--db_allprivs_user $mysql_root \
--db_allprivs_password $mysql_root_passwd \
--db_dba_user $mysql_root \
--db_dba_password $mysql_root_passwd \
--db_app_user $mysql_root \
--db_app_password $mysql_root_passwd \
--db_filtered_user $mysql_root \
--db_filtered_password $mysql_root_passwd \
--grpc_port $grpc_port \
--service_map 'grpc-queryservice,grpc-tabletmanager,grpc-updatestream' \
--pid_file $VTDATAROOT/vttablet.pid \
--vtctld_addr http://$vtctld_host:$vtctld_web_port/ \
--table-acl-config-mode=$table_acl_config_mode \
--disable_active_reparents \
$(if [ -n "$table_acl_config" ]; then echo "--table-acl-config $table_acl_config"; fi) \
$(if [ "$queryserver_config_strict_table_acl" == "true" ]; then echo "--queryserver-config-strict-table-acl"; fi) \
$(if [ "$enforce_tableacl_config" == "true" ]; then echo "--enforce-tableacl-config"; fi) \
--table-acl-config-reload-interval $table_acl_config_reload_interval
EOF