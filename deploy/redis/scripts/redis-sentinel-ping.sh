#!/bin/sh
set -e
if [ ! -z "$SENTINEL_PASSWORD" ]; then
  cmd="redis-cli -h localhost -p 26379 -a $SENTINEL_PASSWORD ping"
else
  cmd="redis-cli -h localhost -p 26379 ping"
fi
response=$(timeout -s 3 $1 $cmd)
if [ $? -eq 124 ]; then
  echo "Timed out"
  exit 1
fi
if [ "$response" != "PONG" ]; then
  echo "$response"
  exit 1
fi