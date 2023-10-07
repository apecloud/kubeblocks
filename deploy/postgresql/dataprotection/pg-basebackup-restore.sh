set -e;
cd ${DP_BACKUP_DIR};
mkdir -p ${DATA_DIR};
# compatible with gzip compression
if [ -f base.tar.gz ];then
  tar -xvf base.tar.gz -C ${DATA_DIR}/;
else
  tar -xvf base.tar -C ${DATA_DIR}/;
fi
if [ -f pg_wal.tar.gz ];then
  tar -xvf pg_wal.tar.gz -C ${DATA_DIR}/pg_wal/;
else
  tar -xvf pg_wal.tar -C ${DATA_DIR}/pg_wal/;
fi
echo "done!";