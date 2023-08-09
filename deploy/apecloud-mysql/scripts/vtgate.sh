#!/bin/bash
. /scripts/set_config_variables.sh
set_config_variables vtgate

cell=${CELL:-'zone1'}
web_port=${VTGATE_WEB_PORT:-'15001'}
grpc_port=${VTGATE_GRPC_PORT:-'15991'}
mysql_server_port=${VTGATE_MYSQL_PORT:-'15306'}
mysql_server_socket_path="/tmp/mysql.sock"

echo "starting vtgate."
su vitess <<EOF
exec vtgate \
  $TOPOLOGY_FLAGS \
  --alsologtostderr \
  --gateway_initial_tablet_timeout $gateway_initial_tablet_timeout \
  --healthcheck_timeout $healthcheck_timeout \
  --srv_topo_timeout $srv_topo_timeout \
  --grpc_keepalive_time $grpc_keepalive_time \
  --grpc_keepalive_timeout $grpc_keepalive_timeout \
  $(if [ "$enable_logs" == "true" ]; then echo "--log_dir $VTDATAROOT"; fi) \
  $(if [ "$enable_query_log" == "true" ]; then echo "--log_queries_to_file $VTDATAROOT/vtgate_querylog.txt"; fi) \
  --port $web_port \
  --grpc_port $grpc_port \
  --mysql_server_port $mysql_server_port \
  --mysql_server_socket_path $mysql_server_socket_path \
  --cell $cell \
  --cells_to_watch $cell \
  --tablet_types_to_wait PRIMARY,REPLICA \
  --tablet_refresh_interval $tablet_refresh_interval \
  --service_map 'grpc-vtgateservice' \
  --pid_file $VTDATAROOT/vtgate.pid \
  --read_write_splitting_policy $read_write_splitting_policy  \
  --read_write_splitting_ratio $read_write_splitting_ratio  \
  --read_after_write_consistency $read_after_write_consistency \
  --read_after_write_timeout $read_after_write_timeout \
  --enable_buffer=$enable_buffer \
  --buffer_size $buffer_size \
  --buffer_window $buffer_window \
  --buffer_max_failover_duration $buffer_max_failover_duration \
  --buffer_min_time_between_failovers $buffer_min_time_between_failovers \
  $(if [ "$mysql_server_require_secure_transport" == "true" ]; then echo "--mysql_server_require_secure_transport"; fi) \
  $(if [ -n "$mysql_server_ssl_cert" ]; then echo "--mysql_server_ssl_cert $mysql_server_ssl_cert"; fi) \
  $(if [ -n "$mysql_server_ssl_key" ]; then echo "--mysql_server_ssl_key $mysql_server_ssl_key"; fi) \
  $(if [ -n "$mysql_auth_server_static_file" ]; then echo "--mysql_auth_server_static_file $mysql_auth_server_static_file"; fi) \
--mysql_auth_server_impl $mysql_auth_server_impl
EOF