#!/bin/bash
set -o errexit
set -e
mkdir -p /home/postgres/pgdata/conf
chmod +777 -R /home/postgres/pgdata/conf
cp /home/postgres/conf/postgresql.conf /home/postgres/pgdata/conf
chmod +777 /home/postgres/pgdata/conf/postgresql.conf
