#!/bin/bash
echo "starting vtctld"
cell=${CELL:-'zone1'}
grpc_port=${VTCTLD_GRPC_PORT:-'15999'}
vtctld_web_port=${VTCTLD_WEB_PORT:-'15000'}
topology_fags=${TOPOLOGY_FLAGS:-'--topo_implementation etcd2 --topo_global_server_address 127.0.0.1:2379 --topo_global_root /vitess/global'}

VTDATAROOT=$VTDATAROOT/vtctld
su vitess <<EOF
mkdir -p $VTDATAROOT
exec vtctld \
$topology_fags \
--alsologtostderr \
--cell $cell \
--service_map 'grpc-vtctl,grpc-vtctld' \
--backup_storage_implementation file \
--file_backup_storage_root $VTDATAROOT/backups \
--log_dir $VTDATAROOT \
--port $vtctld_web_port \
--grpc_port $grpc_port \
--pid_file $VTDATAROOT/vtctld.pid
EOF