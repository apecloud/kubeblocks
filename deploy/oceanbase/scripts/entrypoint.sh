#!/usr/bin/env bash

#
# Copyright (c) 2023 OceanBase
# ob-operator is licensed under Mulan PSL v2.
# You can use this software according to the terms and conditions of the Mulan PSL v2.
# You may obtain a copy of Mulan PSL v2 at:
#          http://license.coscl.org.cn/MulanPSL2
# THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
# EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
# MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
# See the Mulan PSL v2 for more details.
#


source /scripts/bootstrap.sh

RECOVERING="$(is_recovering)"
echo "Recovering: $RECOVERING"

function wait_for_observer_ready {
  until nc -z 127.0.0.1 2881; do
    echo "observer on this node is not ready, wait for a moment..."
    sleep 3
  done
}

# If the server is recovering from crash
if [ $RECOVERING = "True" ]; then
  # If the IP of recovering server changed
  if [ "$(check_if_ip_changed)" = "Changed" ]; then
    echo "IP changed, need to rejoin the cluster"
    clean_dirs
    echo "Prepare config folders"
    prepare_dirs
    echo "Start server"
    start_observer
  else
    echo "IP not changed, use existing configs to start server"
    start_observer_with_exsting_configs
  fi
else
  echo "New machine, need to join the cluster"
  echo "Prepare config folders"
  prepare_dirs
  echo "Start server"
  start_observer
fi

wait_for_observer_ready

if [ $RECOVERING = "True" ]; then
  echo "Resolving other servers' IPs"
  get_pod_ip_list

  echo "Checking cluster health"
  CLUSTER_HEALTHY="$(others_running)"
  echo "Cluster healthy: $CLUSTER_HEALTHY"

  # If the OB Cluster is healthy
  if [ $CLUSTER_HEALTHY = "True" ]; then
    echo "Add this server to cluster"
    add_server
    echo "Delete inactive servers"
    delete_inactive_servers

    # Recover from crash or rolling update, create ready flag at last
    echo "Creating readiness flag..."
    create_ready_flag
  else
    echo "Cluster is not healthy, fail to recover and join the cluster"
  fi
else
  echo "Creating readiness flag..."
  create_ready_flag

  echo "Resolving other servers' IPs"
  get_pod_ip_list

  echo "Checking cluster health"
  CLUSTER_HEALTHY="$(others_running)"
  echo "Cluster healthy: $CLUSTER_HEALTHY"

  # If the OB Cluster is healthy
  if [ $CLUSTER_HEALTHY = "True" ]; then
    echo "Add this server to cluster"
    add_server
    echo "Delete inactive servers"
    delete_inactive_servers
  else
    # If current server is chosen to run RS
    if [ $ORDINAL_INDEX -lt $ZONE_COUNT ]; then
      # Choose the first RS to bootstrap
      if [ $ORDINAL_INDEX -eq 0 ]; then
        echo "Choose the first RS to bootstrap cluster"
        echo "Wait for all Rootservice to be ready"
        bootstrap_obcluster
        if [ $? -eq 0 ]; then
          echo "Bootstrap successfully"
        fi
      else
        echo "Ready to be bootstrapped"
      fi
    else
      echo "Add this server to cluster"
      add_server
    fi
  fi
fi

sleep 3600000000