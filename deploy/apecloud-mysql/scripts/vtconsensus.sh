#!/bin/bash
. /scripts/set_config_variables.sh
set_config_variables vtconsensus

echo "starting vtconsensus"
cell=${CELL:-'zone1'}

vtconsensusport=${VTCONSENSUS_PORT:-'16000'}
topology_fags=${TOPOLOGY_FLAGS:-'--topo_implementation etcd2 --topo_global_server_address 127.0.0.1:2379 --topo_global_root /vitess/global'}

VTDATAROOT=$VTDATAROOT/vtconsensus
su vitess <<EOF
mkdir -p $VTDATAROOT
exec vtconsensus \
  $topology_fags \
  --alsologtostderr \
  --refresh_interval $refresh_interval \
  --scan_repair_timeout $scan_repair_timeout \
  $(if [ "$enable_logs" == "true" ]; then echo "--log_dir $VTDATAROOT"; fi) \
  --db_username "$MYSQL_ROOT_USER" \
  --db_password "$MYSQL_ROOT_PASSWORD"
EOF