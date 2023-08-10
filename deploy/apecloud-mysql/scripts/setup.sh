#!/bin/bash
rmdir /docker-entrypoint-initdb.d && mkdir -p /data/mysql/docker-entrypoint-initdb.d && ln -s /data/mysql/docker-entrypoint-initdb.d /docker-entrypoint-initdb.d;
exec docker-entrypoint.sh
