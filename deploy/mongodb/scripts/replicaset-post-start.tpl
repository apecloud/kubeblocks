#!/bin/sh
# usage: replicaset-post-start.sh type_name is_configsvr
# type_name: component.type, in uppercase
# is_configsvr: true or false, default false
{{- $mongodb_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
{{- $mongodb_port_info := getPortByName ( index $.podSpec.containers 0 ) "mongodb" }}

# require port
{{- $mongodb_port := 27017 }}
{{- if $mongodb_port_info }}
{{- $mongodb_port = $mongodb_port_info.containerPort }}
{{- end }}

PORT={{ $mongodb_port }}
MONGODB_ROOT={{ $mongodb_root }}
INDEX=$(echo $KB_POD_NAME | grep -o "\-[0-9]\+\$");
INDEX=${INDEX#-};
if [ $INDEX -ne 0 ]; then exit 0; fi

until mongosh --quiet --port $PORT --eval "print('ready')"; do sleep 1; done

RPL_SET_NAME=$(echo $KB_POD_NAME | grep -o ".*-");
RPL_SET_NAME=${RPL_SET_NAME%-};

TYPE_NAME=$1
IS_CONFIGSVR=$2
MEMBERS=""
i=0
while [ $i -lt $(eval echo \$KB_"$TYPE_NAME"_N) ]; do
  host=$(eval echo \$KB_"$TYPE_NAME"_"$i"_HOSTNAME)
  host=$host"."$KB_NAMESPACE".svc.cluster.local"
  until mongosh --quiet --port $PORT --host $host --eval "print('peer is ready')"; do sleep 1; done
  if [ $i -eq 0 ]; then 
    MEMBERS="{_id: $i, host: \"$host:$PORT\", priority:2}"
  else 
    MEMBERS="$MEMBERS,{_id: $i, host: \"$host:$PORT\"}"
  fi
  i=$(( i + 1))
done
CONFIGSVR=""
if [ ""$IS_CONFIGSVR = "true" ]; then CONFIGSVR="configsvr: true,"; fi

until is_inited=$(mongosh --quiet --port $PORT --eval "rs.status().ok" -u root --password $MONGODB_ROOT_PASSWORD || mongosh --quiet --port $PORT --eval "try { rs.status().ok } catch (e) { 0 }") ; do sleep 1; done
if [ $is_inited -eq 1 ]; then
  exit 0
fi;
sleep 10
set -e
mongosh --quiet --port $PORT --eval "rs.initiate({_id: \"$RPL_SET_NAME\", $CONFIGSVR members: [$MEMBERS]})";
set +e

(until mongosh --quiet --port $PORT --eval "rs.isMaster().isWritablePrimary"|grep true; do sleep 1; done;
echo "create user";
mongosh --quiet --port $PORT admin --eval "db.createUser({ user: \"$MONGODB_ROOT_USER\", pwd: \"$MONGODB_ROOT_PASSWORD\", roles: [{role: 'root', db: 'admin'}] })") </dev/null  >/dev/null 2>&1 &
