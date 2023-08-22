#!/bin/bash
set -ex

if [ ! -z $MYSQL_ROOT_PASSWORD ]; then
  password_flag="-p$MYSQL_ROOT_PASSWORD"
fi
global_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select IP_PORT from information_schema.wesql_cluster_global order by MATCH_INDEX desc;"`
global_info_arr=($global_info)
leader_ip_port=$KB_CONSENSUS_LEADER_POD_FQDN:13306
try_times=10

for((i=1;i<${#global_info_arr[@]};i++)) do
  if [ "$leader_ip_port" == "${global_info_arr[i]}" ];then
    echo "do not transfer to leader, leader:${global_info_arr[i]}"
  else
    # get currently leader weight
    leader_weight_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ELECTION_WEIGHT from information_schema.wesql_cluster_global where IP_PORT='$leader_ip_port' limit 1;"`
    leader_weight_arr=($leader_weight_info)
    leader_weight=${leader_weight_arr[1]}
    echo "leader_weight=$leader_weight"

    # get current follower weight
    current_follower_weight_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ELECTION_WEIGHT from information_schema.wesql_cluster_global where IP_PORT='${global_info_arr[i]}' limit 1;"`
    current_follower_weight_arr=($current_follower_weight_info)
    current_follower_weight=${current_follower_weight_arr[1]}
    echo "current_follower_weight=$current_follower_weight"
    if [ $current_follower_weight -lt $leader_weight ];then
       echo "leader weight larger than follower weight, update leader weight to current follower weight! leader:$leader_ip_port, leader weight:$leader_weight, follower:${global_info_arr[i]}, follower weight:$current_follower_weight"
       mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.configure_follower('$leader_ip_port', $current_follower_weight, 0);" 2>&1
    fi
    mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.change_leader('${global_info_arr[i]}');" 2>&1
    sleep 5
    role_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ROLE from information_schema.wesql_cluster_local;"`
    role_info_arr=($role_info)
    real_role=${role_info_arr[1]}
    if [ "$real_role" == "Follower" ];then
      echo "transfer successfully"
      new_leader_host_and_port=${global_info_arr[i]}
      new_leader_host=${new_leader_host_and_port%%:*}
      echo "new_leader_host=$new_leader_host"
      break
    fi
  fi
  ((try_times--))
  if [ $try_times -le 0 ];then
    break
  fi
done