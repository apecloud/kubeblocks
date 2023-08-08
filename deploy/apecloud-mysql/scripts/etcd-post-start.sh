#!/bin/bash
etcd_port=${ETCD_PORT:-'2379'}
etcd_server=${ETCD_SERVER:-'127.0.0.1'}

cell=${CELL:-'zone1'}
export ETCDCTL_API=2

etcdctl --endpoints "http://127.0.0.1:${etcd_port}" get "/vitess/global" >/dev/null 2>&1
if [[ $? -eq 1 ]]; then
  exit 0
fi

echo "add /vitess/global"
etcdctl --endpoints "http://127.0.0.1:${etcd_port}" mkdir /vitess/global

echo "add /vitess/$cell"
etcdctl --endpoints "http://127.0.0.1:${etcd_port}" mkdir /vitess/$cell

# And also add the CellInfo description for the cell.
# If the node already exists, it's fine, means we used existing data.
echo "add $cell CellInfo"
set +e
vtctl --topo_implementation etcd2 \
  --topo_global_server_address "127.0.0.1:${etcd_port}" \
  --topo_global_root /vitess/global VtctldCommand AddCellInfo \
  --root /vitess/$cell \
  --server-address "${etcd_server}:${etcd_port}" \
  $cell