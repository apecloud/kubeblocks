#!/bin/bash
if [ ! -z $MYSQL_ROOT_PASSWORD ]; then
  password_flag="-p$MYSQL_ROOT_PASSWORD"
fi
mysql -h$KB_CONSENSUS_LEADER_POD_FQDN -uroot $password_flag -e "call dbms_consensus.change_leader('$KB_SWITCHOVER_CANDIDATE_FQDN:13306');"