#!/bin/sh
set -ex
echo "Waiting for taosAdapter service to be ready..."
until curl -L -u ${TAOS_ADAPTER_USERNAME}:${TAOS_ADAPTER_PASSWORD}   -d "select name, ntables, status from information_schema.ins_databases;"   localhost:${TAOS_ADAPTER_PORT}/rest/sql; do sleep 5; done
echo "Start taosAdapter service succeeded!"