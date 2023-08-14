#!/bin/sh
set -ex
if [ ! -z "$REDIS_DEFAULT_PASSWORD" ]; then
  redis-cli -h 127.0.0.1 -p 6379 -a "$REDIS_DEFAULT_PASSWORD" acl save
else
  redis-cli -h 127.0.0.1 -p 6379 acl save
fi
