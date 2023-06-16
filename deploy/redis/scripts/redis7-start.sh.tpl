#!/bin/sh
set -ex
echo "include /etc/conf/redis.conf" >> /etc/redis/redis.conf
echo "replica-announce-ip $KB_POD_FQDN" >> /etc/redis/redis.conf
echo "masteruser $REDIS_REPL_USER" >> /etc/redis/redis.conf
echo "masterauth $REDIS_REPL_PASSWORD" >> /etc/redis/redis.conf
{{- $data_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
touch {{ $data_root }}/users.acl
echo "user default on allcommands allkeys >$REDIS_DEFAULT_PASSWORD" > /data/users.acl
echo "user $REDIS_REPL_USER on +psync +replconf +ping >$REDIS_REPL_PASSWORD" >> /data/users.acl
echo "user $REDIS_SENTINEL_USER on allchannels +multi +slaveof +ping +exec +subscribe +config|rewrite +role +publish +info +client|setname +client|kill +script|kill >$REDIS_SENTINEL_PASSWORD" >> /data/users.acl
echo "aclfile /data/users.acl" >> /etc/redis/redis.conf
exec redis-server /etc/redis/redis.conf \
--loadmodule /opt/redis-stack/lib/redisearch.so ${REDISEARCH_ARGS} \
--loadmodule /opt/redis-stack/lib/redisgraph.so ${REDISGRAPH_ARGS} \
--loadmodule /opt/redis-stack/lib/redistimeseries.so ${REDISTIMESERIES_ARGS} \
--loadmodule /opt/redis-stack/lib/rejson.so ${REDISJSON_ARGS} \
--loadmodule /opt/redis-stack/lib/redisbloom.so ${REDISBLOOM_ARGS}