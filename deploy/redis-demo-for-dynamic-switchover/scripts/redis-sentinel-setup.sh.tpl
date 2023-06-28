#!/bin/sh
set -ex
cat>/etc/sentinel/redis-sentinel.conf<<EOF
port 26379
sentinel resolve-hostnames yes
sentinel announce-hostnames yes
sentinel monitor $REDIS_SVC_NAME $REDIS_PRIMAY_HOST 6379 2
sentinel down-after-milliseconds $REDIS_SVC_NAME 5000
sentinel failover-timeout $REDIS_SVC_NAME 60000
sentinel parallel-syncs $REDIS_SVC_NAME 1
{{- /* $primary_svc := printf "%s-%s.%s.svc" $clusterName $redis_component.name $namespace */}}
EOF