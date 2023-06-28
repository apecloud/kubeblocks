#!/bin/sh
set -ex
echo "Waiting for taosAdapter service to be ready..."
until curl -L -u roor:taosdata   -d "select name, ntables, status from information_schema.ins_databases;"   localhost:6041/rest/sql; do sleep 5; done
echo "Start taosAdapter service succeeded!"