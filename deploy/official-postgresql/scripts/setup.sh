#!/bin/bash
set -o errexit
set -o nounset

. /kb-scripts/libpostgresql.sh

export POSTGRESQL_INIT_MAX_TIMEOUT="${POSTGRESQL_INIT_MAX_TIMEOUT:-60}"
export POSTGRESQL_DAEMON_USER="postgres"
export POSTGRESQL_DAEMON_GROUP="postgres"
export POSTGRESQL_BIN_DIR="/usr/bin"
export POSTGRESQL_CONF_FILE="/kubeblocks/conf/postgresql.conf"
KB_0_POD_NAME_PREFIX="${KB_0_HOSTNAME%%\.*}"

# configmap readonly
if [ ! -d "/kubeblocks/conf" ];then
  cp -r /var/lib/postgresql/conf kubeblocks/
fi

# default secondary when pgdata is not empty
if [ "$(ls -A ${PGDATA})" ]; then
  touch "$PGDATA"/standby.signal
else
  if [ "$KB_0_POD_NAME_PREFIX" != "$KB_POD_NAME" ]; then
    export POSTGRESQL_MASTER_HOST=$KB_0_HOSTNAME
    export POSTGRESQL_MASTER_PORT_NUMBER="5432"
    # Ensure 'daemon' user exists when running as 'root'
    am_i_root && ensure_user_exists "$POSTGRESQL_DAEMON_USER" --group "$POSTGRESQL_DAEMON_GROUP"
    postgresql_slave_init_db
    touch "$PGDATA"/standby.signal
  fi
fi
docker-entrypoint.sh --config-file=/kubeblocks/conf/postgresql.conf --hba_file=/kubeblocks/conf/pg_hba.conf