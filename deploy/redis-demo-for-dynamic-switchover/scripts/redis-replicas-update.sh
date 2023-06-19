#!/bin/sh
set -ex
REDISCLI="/kb/tools/redis-cli"

#input param_name param_value

replica_name=shift
replica=shift

echo "update replica $replica_name: $replica"
