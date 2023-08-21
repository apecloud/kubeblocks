#!/bin/bash
set -ex

if [ ! -z $MYSQL_ROOT_PASSWORD ]; then
  password_flag="-p$MYSQL_ROOT_PASSWORD"
fi

leader_ip_port=$KB_CONSENSUS_LEADER_POD_FQDN:13306
candidate_ip_port=$KB_SWITCHOVER_CANDIDATE_FQDN:13306

# get currently leader weight
leader_weight_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ELECTION_WEIGHT from information_schema.wesql_cluster_global where IP_PORT='$leader_ip_port' limit 1;"`
leader_weight_arr=($leader_weight_info)
leader_weight=${leader_weight_arr[1]}
echo "leader_weight=$leader_weight"

# get candidate weight
candidate_weight_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ELECTION_WEIGHT from information_schema.wesql_cluster_global where IP_PORT='$candidate_ip_port' limit 1;"`
candidate_weight_arr=($candidate_weight_info)
candidate_weight=${candidate_weight_arr[1]}
echo "candidate_weight=$leader_weight"

# if candidate weight is less than leader weight, update leader weight to candidate weight
if [ $candidate_weight -lt $leader_weight ];then
   echo "leader weight larger than follower weight, update leader weight to current follower weight! leader:$leader_ip_port, leader weight:$leader_weight, follower:$candidate_ip_port, follower weight:$candidate_weight"
   mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.configure_follower('$leader_ip_port', $candidate_weight, 0);" 2>&1
fi

# do switchover
mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.change_leader('$KB_SWITCHOVER_CANDIDATE_FQDN:13306');"

sleep 5

# check if switchover successfully
role_info=`mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag 2>/dev/null -e "select ROLE from information_schema.wesql_cluster_local;"`
role_info_arr=($role_info)
real_role=${role_info_arr[1]}
if [ "$real_role" == "Follower" ];then
  echo "switchover successfully"
else
  echo "switchover failed, please check!"
  exit 1
fi