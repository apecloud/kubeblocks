#!/bin/sh
set -ex
echo "Waiting for redis service $REDIS_SVC_NAME to be ready..."
until redis-cli -h $REDIS_SVC_NAME -p $REDIS_SVC_PORT ping; do sleep 1; done
echo "redis service ready, Starting sentinel..."
echo "sentinel announce-ip $KB_POD_FQDN" >> /etc/sentinel/redis-sentinel.conf
exec redis-server /etc/sentinel/redis-sentinel.conf --sentinel
echo "Start sentinel succeeded!"