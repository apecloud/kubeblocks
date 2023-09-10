#!/bin/bash

BASE_BACKUP_TIME=${BASE_BACKUP_START_TIME}
if [ -f $DATA_DIR/xtrabackup_info ]; then
  BASE_BACKUP_TIME=$(cat $DATA_DIR/xtrabackup_info | grep start_time | awk -F ' = ' '{print $2}');
  BASE_BACKUP_TIME=$(date -d"${BASE_BACKUP_TIME}" -u '+%Y-%m-%dT%H:%M:%SZ')
fi
log_index_name="archive_log.index"

function fetch_pitr_binlogs() {
    cd ${BACKUP_DIR}
    echo "INFO: fetch binlogs from ${BASE_BACKUP_TIME}"
    kb_recovery_timestamp=$(date -d "${KB_RECOVERY_TIME}" +%s)
    for file in $(find . -newermt "${BASE_BACKUP_TIME}" -type f -exec ls -tr {} + | grep .zst );do
        file_path=${file#./}
        file_without_zst=${file_path%.*}
        dir_path=`dirname ${file_path}`
        # mkdir the log directory
        mkdir -p ${PITR_DIR}/${dir_path}
        zstd -d ${file} -o ${PITR_DIR}/${file_without_zst}
        echo "${PITR_RELATIVE_PATH}/${file_without_zst}" >> ${PITR_DIR}/${log_index_name}
        # check if the binlog file contains the data for recovery time
        log_start_time=$(mysqlbinlog ${PITR_DIR}/${file_without_zst} | grep -m 1 "end_log_pos" | awk '{print $1, $2}'|tr -d '#')
        log_start_timestamp=$(date -d "${log_start_time}" +%s)
        if [[ ${log_start_timestamp} -gt ${kb_recovery_timestamp} ]];then
           break
        fi
    done
}

function save_to_restore_file() {
    if [ -f ${DATA_DIR}/.xtrabackup_restore_new_cluster ];then
       restore_signal_file=${DATA_DIR}/.xtrabackup_restore_new_cluster
    else
       restore_signal_file=${DATA_DIR}/.restore_new_cluster
    fi
    echo "archive_log_index=${PITR_RELATIVE_PATH}/${log_index_name}" > ${restore_signal_file}
    kb_recover_time=$(date -d "${KB_RECOVERY_TIME}" -u '+%Y-%m-%d %H:%M:%S')
    echo "recovery_target_datetime=${kb_recover_time}" >> ${restore_signal_file}
    sync
}

fetch_pitr_binlogs

if [ -f ${PITR_DIR}/${log_index_name} ];then
  save_to_restore_file
  echo "INFO: fetch binlog finished."
else
  echo "INFO: didn't get any binlogs."
fi
