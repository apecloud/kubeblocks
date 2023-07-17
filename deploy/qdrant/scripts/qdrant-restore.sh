#!/usr/bin/env bash

set -e
mkdir -p ${DATA_DIR}
res=`ls -A ${DATA_DIR}`
if [ ! -z "${res}" ]; then
  echo "${DATA_DIR} is not empty! Please make sure that the directory is empty before restoring the backup."
  exit 1
fi

# start qdrant restore process
qdrant --storage-snapshot ${BACKUP_DIR}/${BACKUP_NAME}  --config-path /qdrant/config/config.yaml  --force-snapshot --uri http://localhost:6333 &

# wait until restore finished
until curl http://localhost:6333/cluster; do sleep 1; done

# restore finished, we can kill the restore process now
pid=`pidof qdrant`
kill -s INT ${pid}
wait ${pid}
