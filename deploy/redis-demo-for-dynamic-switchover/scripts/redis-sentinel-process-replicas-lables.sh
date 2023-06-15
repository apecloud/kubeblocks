#!/bin/sh
set -ex

# REDISCLI="/kb/tools/redis-cli"

replica=$(cat $1)

echo "current replica is $replica"