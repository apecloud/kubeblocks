
if [ -d ${DATA_DIR}.old ];
  then echo "${DATA_DIR}.old directory already exists, skip restore.";
  exit 0;
fi

mkdir -p ${PITR_DIR};

latest_wal=$(ls ${DATA_DIR}/pg_wal -lI "*.history" | grep ^- | awk '{print $9}' | sort | tail -n 1)
start_wal_log=`basename $latest_wal`

echo "fetch-wal-log ${BACKUP_DIR} ${PITR_DIR} ${start_wal_log} \"${KB_RECOVERY_TIME}\" true"
fetch-wal-log ${BACKUP_DIR} ${PITR_DIR} ${start_wal_log} "${KB_RECOVERY_TIME}" true

chmod 777 -R ${PITR_DIR};
touch ${DATA_DIR}/recovery.signal;
mkdir -p ${CONF_DIR};
chmod 777 -R ${CONF_DIR};
mkdir -p ${RESTORE_SCRIPT_DIR};
echo "#!/bin/bash" > ${RESTORE_SCRIPT_DIR}/kb_restore.sh;
echo "[[ -d '${DATA_DIR}.old' ]] && mv -f ${DATA_DIR}.old/* ${DATA_DIR}/;" >> ${RESTORE_SCRIPT_DIR}/kb_restore.sh;
echo "sync;" >> ${RESTORE_SCRIPT_DIR}/kb_restore.sh;
chmod +x ${RESTORE_SCRIPT_DIR}/kb_restore.sh;
echo "restore_command='case "%f" in *history) cp ${PITR_DIR}/%f %p ;; *) mv ${PITR_DIR}/%f %p ;; esac'" > ${CONF_DIR}/recovery.conf;
echo "recovery_target_time='${KB_RECOVERY_TIME}'" >> ${CONF_DIR}/recovery.conf;
echo "recovery_target_action='promote'" >> ${CONF_DIR}/recovery.conf;
echo "recovery_target_timeline='latest'" >> ${CONF_DIR}/recovery.conf;
mv ${DATA_DIR} ${DATA_DIR}.old;
echo "done.";
sync;