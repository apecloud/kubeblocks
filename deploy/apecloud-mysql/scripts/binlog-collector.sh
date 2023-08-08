#!/bin/bash
set -e;
LOG_DIR=/data/mysql/data;
cd $LOG_DIR;
LOG_START_TIME=$(mysqlbinlog $(ls -Ftr $LOG_DIR/|grep -e '^mysql-bin.*[[:digit:]]$'|head -n 1) |grep 'Xid =' |head -n 1|awk -F ' server id ' '{print $1}'|tr -d '#')
for i in $(ls -Ft $LOG_DIR/|grep -e '^mysql-bin.*[[:digit:]]$'); do LOG_STOP_TIME=$(mysqlbinlog $i |grep 'Xid =' |tail -n 1|awk -F ' server id ' '{print $1}'|tr -d '#'); [[ "$LOG_STOP_TIME" != "" ]] && break;  done
if [ "${LOG_START_TIME}" == "" ]; then LOG_START_TIME=${LOG_STOP_TIME}; fi
LOG_START_TIME=$(date -d "$LOG_START_TIME" -u '+%Y-%m-%dT%H:%M:%SZ')
LOG_STOP_TIME=$(date -d "$LOG_STOP_TIME" -u '+%Y-%m-%dT%H:%M:%SZ')
printf "{\"startTime\": \"$LOG_START_TIME\" ,\"stopTime\": \"$LOG_STOP_TIME\"}"