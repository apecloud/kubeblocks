#!/bin/bash
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
    mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.change_leader('${global_info_arr[i]}');" 2>&1
    sleep 1
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