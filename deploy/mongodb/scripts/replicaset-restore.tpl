#!/bin/sh

set -e
PORT=27017
MONGODB_ROOT=/data/mongodb
mkdir -p $MONGODB_ROOT/db
mkdir -p $MONGODB_ROOT/logs
mkdir -p $MONGODB_ROOT/tmp

res=`ls -A ${DATA_DIR}`
if [ ! -z ${res} ]; then
  echo "${DATA_DIR} is not empty! Please make sure that the directory is empty before restoring the backup."
  exit 1
fi
tar -xvf ${BACKUP_DIR}/${BACKUP_NAME}.tar.gz -C ${DATA_DIR}/../
mv ${DATA_DIR}/../${BACKUP_NAME}/* ${DATA_DIR}
RPL_SET_NAME=$(echo $KB_POD_NAME | grep -o ".*-");
RPL_SET_NAME=${RPL_SET_NAME%-};
MODE=$1
mongod $MODE --bind_ip_all --port $PORT --dbpath $MONGODB_ROOT/db --directoryperdb --logpath $MONGODB_ROOT/logs/mongodb.log  --logappend --pidfilepath $MONGODB_ROOT/tmp/mongodb.pid&
export CLIENT=`which mongosh>/dev/null&&echo mongosh||echo mongo`
until $CLIENT --quiet --port $PORT --host $host --eval "print('peer is ready')"; do sleep 1; done
PID=`cat $MONGODB_ROOT/tmp/mongodb.pid`

$CLIENT --quiet --port $PORT local --eval "db.system.replset.deleteOne({})"
$CLIENT --quiet --port $PORT local --eval "db.system.replset.find()"
$CLIENT --quiet --port $PORT admin --eval 'db.dropUser("root", {w: "majority", wtimeout: 4000})' || true
kill $PID
wait $PID
