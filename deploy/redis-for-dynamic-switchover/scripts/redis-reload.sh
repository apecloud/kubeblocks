#!/bin/sh
set -ex
REDISCLI="/kb/tools/redis-cli"

$REDISCLI -h 127.0.0.1 -p 6379 CONFIG SET "$@"