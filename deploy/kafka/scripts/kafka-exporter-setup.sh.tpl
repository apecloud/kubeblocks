#!/bin/bash
tlsconfig=""
# if kafka_tls equals true
if [ $KAFKA_TLS = "true" ]; then
  tlsconfig="--tls.enabled"
fi
exec kafka_exporter --web.listen-address=:9308 --kafka.server=$KAFKA_SVC_NAME:$KAFKA_SVC_PORT $tlsconfig
