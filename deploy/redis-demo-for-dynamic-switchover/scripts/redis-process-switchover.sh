#!/bin/sh
set -ex

REDISCLI="/kb/tools/redis-cli"

current_role=$(cat $1)
echo "current pod changed to $current_role"

#appendonly yes
value="no"

if [ "x$current_role" == "xprimary" ];then
value="yes"
fi

$REDISCLI -h 127.0.0.1 -p 6379 CONFIG SET appendonly "$value"
