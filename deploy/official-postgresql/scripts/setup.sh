#!/bin/bash
set -o errexit
set -o nounset

. /kb-scripts/libpostgresql.sh

export POSTGRESQL_INIT_MAX_TIMEOUT="${POSTGRESQL_INIT_MAX_TIMEOUT:-60}"
export POSTGRESQL_BIN_DIR="/usr/bin"
export POSTGRESQL_CONF_DIR="/kubeblocks/conf"
export POSTGRESQL_CONF_FILE="/kubeblocks/conf/postgresql.conf"
export POSTGRESQL_MASTER_HOST=$KB_0_HOSTNAME
KB_0_POD_NAME_PREFIX="${KB_0_HOSTNAME%%\.*}"

# configmap readonly
if [ ! -d "$POSTGRESQL_CONF_DIR" ];then
  cp -r /var/lib/postgresql/conf kubeblocks/
fi

# default standby when pgdata is not empty
if [ "$(ls -A ${PGDATA})" ]; then
  touch "$PGDATA"/standby.signal
else
  if [ "$KB_0_POD_NAME_PREFIX" != "$KB_POD_NAME" ]; then
    # Ensure 'daemon' user exists when running as 'root'
    am_i_root && ensure_user_exists "$POSTGRES_USER"
    postgresql_slave_init_db
    primary_conninfo="host=$KB_0_HOSTNAME port=$POSTGRESQL_MASTER_PORT_NUMBER user=$PGUSER password=$PGPASSWORD application_name=$KB_POD_NAME"
    postgresql_set_property "primary_conninfo" "$primary_conninfo" "$POSTGRESQL_CONF_FILE"
    touch "$PGDATA"/standby.signal
  fi
fi
docker-entrypoint.sh --config-file="$POSTGRESQL_CONF_FILE" --hba_file=/kubeblocks/conf/pg_hba.conf