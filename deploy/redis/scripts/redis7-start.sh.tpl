#!/bin/sh
set -ex
echo "include /etc/conf/redis.conf" >> /etc/redis/redis.conf
echo "replica-announce-ip $KB_POD_FQDN" >> /etc/redis/redis.conf
echo "masteruser $REDIS_REPL_USER" >> /etc/redis/redis.conf
echo "masterauth $REDIS_REPL_PASSWORD" >> /etc/redis/redis.conf
{{- $data_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
if [ -f /data/users.acl ]; then
  sed -i "/user default on/d" /data/users.acl
  sed -i "/user $REDIS_REPL_USER on/d" /data/users.acl
  sed -i "/user $REDIS_SENTINEL_USER on/d" /data/users.acl
else
  touch /data/users.acl
fi
echo "user default on allcommands allkeys >$REDIS_DEFAULT_PASSWORD" >> /data/users.acl
echo "user $REDIS_REPL_USER on +psync +replconf +ping >$REDIS_REPL_PASSWORD" >> /data/users.acl
echo "user $REDIS_SENTINEL_USER on allchannels +multi +slaveof +ping +exec +subscribe +config|rewrite +role +publish +info +client|setname +client|kill +script|kill >$REDIS_SENTINEL_PASSWORD" >> /data/users.acl
echo "aclfile /data/users.acl" >> /etc/redis/redis.conf

# usage: retry <command>
retry() {
  local max_attempts=20
  local attempt=1
  until "$@" || [ $attempt -eq $max_attempts ]; do
    echo "Command '$*' failed. Attempt $attempt of $max_attempts. Retrying in 5 seconds..."
    attempt=$((attempt + 1))
    sleep 3
  done
  if [ $attempt -eq $max_attempts ]; then
    echo "Command '$*' failed after $max_attempts attempts. Exiting..."
    exit 1
  fi
}

start_redis_server() {
    exec redis-server /etc/redis/redis.conf \
    --loadmodule /opt/redis-stack/lib/redisearch.so ${REDISEARCH_ARGS} \
    --loadmodule /opt/redis-stack/lib/redisgraph.so ${REDISGRAPH_ARGS} \
    --loadmodule /opt/redis-stack/lib/redistimeseries.so ${REDISTIMESERIES_ARGS} \
    --loadmodule /opt/redis-stack/lib/rejson.so ${REDISJSON_ARGS} \
    --loadmodule /opt/redis-stack/lib/redisbloom.so ${REDISBLOOM_ARGS}
}

create_replication() {
    # Waiting for primary pod information from the DownwardAPI annotation to be available, with a maximum of 5 attempts
    attempt=1
    max_attempts=20
    while [ $attempt -le $max_attempts ] && [ -z "$(cat /kb-podinfo/primary-pod)" ]; do
      echo "Waiting for primary pod information from the DownwardAPI annotation to be available, attempt $attempt of $max_attempts..."
      sleep 5
      attempt=$((attempt + 1))
    done
    primary=$(cat /kb-podinfo/primary-pod)
    echo "DownwardAPI get primary=$primary" >> /etc/redis/.kb_set_up.log
    echo "KB_POD_NAME=$KB_POD_NAME" >> /etc/redis/.kb_set_up.log
    if [ -z "$primary" ]; then
      echo "Primary pod information not available. shutdown redis-server..."
      redis-cli -h 127.0.0.1 -p 6379 -a $REDIS_DEFAULT_PASSWORD shutdown
      exit 1
    fi
    if [ "$primary" = "$KB_POD_NAME" ]; then
      echo "primary instance skip create a replication relationship."
    else
      primary_fqdn="$primary.$KB_CLUSTER_NAME-$KB_COMP_NAME-headless.$KB_NAMESPACE.svc"
      echo "primary_fqdn=$primary_fqdn" >> /etc/redis/.kb_set_up.log
      retry redis-cli -h $primary_fqdn -p 6379 -a $REDIS_DEFAULT_PASSWORD ping
      redis-cli -h 127.0.0.1 -p 6379 -a $REDIS_DEFAULT_PASSWORD replicaof $primary_fqdn 6379
      if [ $? -ne 0 ]; then
        echo "Failed to create a replication relationship. shutdown redis-server..."
        redis-cli -h 127.0.0.1 -p 6379 -a $REDIS_DEFAULT_PASSWORD shutdown
      fi
    fi
}

create_replication &
start_redis_server