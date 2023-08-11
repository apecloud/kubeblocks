function fetch-wal-log(){
    backup_log_dir=$1
    wal_destination_dir=$2
    start_wal_name=$3
    restore_time=`date -d "$4" +%s`
    pitr=$5
    echo "PITR: $pitr"

    if [[ ! -d ${backup_log_dir} ]]; then
       echo "ERROR: ${backup_log_dir} not exists"
       exit 1
    fi

    exit_fetch_wal=0 && mkdir -p $wal_destination_dir
    for dir_name in $(ls ${backup_log_dir} -l | grep ^d | awk '{print $9}' | sort); do
      if [[ $exit_fetch_wal -eq 1 ]]; then
         exit 0
      fi

      cd ${backup_log_dir}/${dir_name}
      # check if the latest_wal_log after the start_wal_log
      latest_wal=$(ls | sort | tail -n 1)
      if [[ $latest_wal < $start_wal_name ]]; then
         continue
      fi

      echo "INFO: start to fetch wal logs from ${backup_log_dir}/${dir_name}"
      for file in $(ls | sort | grep ".gz"); do
         if [[ $file < $start_wal_name ]]; then
            continue
         fi
         if [[ $pitr != "true" && $file =~ ".history"  ]]; then
            # if not restored for pitr, only fetch the current timeline log
            echo "INFO: exit for new timeline."
            exit_fetch_wal=1
            break
         fi

         if [ ! -f $file ]; then
            echo "ERROR: $file was deleted during fetching the wal log. Please try again!"
            exit 1
         fi
         wal_name=${file%.*}
         echo "INFO: copying $wal_name"
         gunzip -c $file > ${wal_destination_dir}/$wal_name

         # check if the wal_log contains the restore_time logs. if ture, stop fetching
         latest_commit_time=$(pg_waldump ${wal_destination_dir}/$wal_name --rmgr=Transaction 2>/dev/null |tail -n 1|awk -F ' COMMIT ' '{print $2}'|awk -F ';' '{print $1}')
         timestamp=`date -d "$latest_commit_time" +%s`
         if [[ $latest_commit_time != "" && $timestamp > $restore_time ]]; then
            echo "INFO: exit when reaching the target time log."
            exit_fetch_wal=1
            break
         fi
      done
    done
}