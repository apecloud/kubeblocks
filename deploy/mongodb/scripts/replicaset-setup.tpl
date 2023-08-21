#!/bin/sh

{{- $mongodb_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
{{- $mongodb_port_info := getPortByName ( index $.podSpec.containers 0 ) "mongodb" }}

# require port
{{- $mongodb_port := 27017 }}
{{- if $mongodb_port_info }}
{{- $mongodb_port = $mongodb_port_info.containerPort }}
{{- end }}

PORT={{ $mongodb_port }}
MONGODB_ROOT={{ $mongodb_root }}
RPL_SET_NAME=$(echo $KB_POD_NAME | grep -o ".*-");
RPL_SET_NAME=${RPL_SET_NAME%-};
mkdir -p $MONGODB_ROOT/db
mkdir -p $MONGODB_ROOT/logs
mkdir -p $MONGODB_ROOT/tmp

BACKUPFILE=$MONGODB_ROOT/db/mongodb.backup
PORT_FOR_RESTORE=27027
if [ -f $BACKUPFILE ]
then
  mongod --bind_ip_all --port $PORT_FOR_RESTORE --dbpath $MONGODB_ROOT/db --directoryperdb --logpath $MONGODB_ROOT/logs/mongodb.log  --logappend --pidfilepath $MONGODB_ROOT/tmp/mongodb.pid&
  until mongosh --quiet --port $PORT_FOR_RESTORE --host $host --eval "print('restore process is ready')"; do sleep 1; done
  PID=`cat $MONGODB_ROOT/tmp/mongodb.pid`

  mongosh --quiet --port $PORT_FOR_RESTORE local --eval "db.system.replset.deleteOne({})"
  mongosh --quiet --port $PORT_FOR_RESTORE local --eval "db.system.replset.find()"
  mongosh --quiet --port $PORT_FOR_RESTORE admin --eval 'db.dropUser("root", {w: "majority", wtimeout: 4000})' || true
  kill $PID
  wait $PID
  rm $BACKUPFILE
fi

exec mongod  --bind_ip_all --port $PORT --replSet $RPL_SET_NAME  --config /etc/mongodb/mongodb.conf
