#!/bin/sh
set -ex
echo "include /etc/conf/redis.conf" >> /etc/redis/redis.conf
echo "replica-announce-ip $KB_POD_FQDN" >> /etc/redis/redis.conf
{{- $data_root := getVolumePathByName ( index $.podSpec.containers 0 ) "data" }}
touch {{ $data_root }}/users.acl
echo "aclfile /data/users.acl" >> /etc/redis/redis.conf
exec redis-server /etc/redis/redis.conf \
--loadmodule /opt/redis-stack/lib/redisearch.so ${REDISEARCH_ARGS} \
--loadmodule /opt/redis-stack/lib/redisgraph.so ${REDISGRAPH_ARGS} \
--loadmodule /opt/redis-stack/lib/redistimeseries.so ${REDISTIMESERIES_ARGS} \
--loadmodule /opt/redis-stack/lib/rejson.so ${REDISJSON_ARGS} \
--loadmodule /opt/redis-stack/lib/redisbloom.so ${REDISBLOOM_ARGS}