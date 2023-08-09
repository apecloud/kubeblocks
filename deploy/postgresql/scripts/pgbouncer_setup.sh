#!/bin/bash
set -o errexit
set -e
mkdir -p /opt/bitnami/pgbouncer/conf/ /opt/bitnami/pgbouncer/logs/ /opt/bitnami/pgbouncer/tmp/
cp /home/pgbouncer/conf/pgbouncer.ini /opt/bitnami/pgbouncer/conf/
echo "\"$POSTGRESQL_USERNAME\" \"$POSTGRESQL_PASSWORD\"" > /opt/bitnami/pgbouncer/conf/userlist.txt
echo -e "\\n[databases]" >> /opt/bitnami/pgbouncer/conf/pgbouncer.ini
echo "postgres=host=$KB_POD_IP port=5432 dbname=postgres" >> /opt/bitnami/pgbouncer/conf/pgbouncer.ini
chmod +777 /opt/bitnami/pgbouncer/conf/pgbouncer.ini
chmod +777 /opt/bitnami/pgbouncer/conf/userlist.txt
useradd pgbouncer
chown -R pgbouncer:pgbouncer /opt/bitnami/pgbouncer/conf/ /opt/bitnami/pgbouncer/logs/ /opt/bitnami/pgbouncer/tmp/
/opt/bitnami/scripts/pgbouncer/run.sh
