#!/bin/bash
set -o errexit
set -o nounset

. /scripts/libsetup.sh

export POSTGRESQL_INIT_MAX_TIMEOUT="${POSTGRESQL_INIT_MAX_TIMEOUT:-60}"
export POSTGRESQL_DAEMON_USER="postgres"
export POSTGRESQL_DAEMON_GROUP="postgres"
export POSTGRESQL_BIN_DIR="/usr/local/bin"
export POSTGRESQL_DATA_DIR="/postgresql/data"
export POSTGRESQL_CONF_FILE="/kubeblocks/conf/postgresql.conf"
KB_0_POD_NAME_PREFIX="${KB_0_HOSTNAME%%\.*}"

# configmap readonly
if [ ! -d "/kubeblocks/conf" ];then
  cp -r postgresql/conf/ kubeblocks/
fi

# default secondary when pgdata is not empty
if [ -d ${PGDATA} ]; then
  touch "$POSTGRESQL_DATA_DIR"/standby.signal
else
  if [ "$KB_0_POD_NAME_PREFIX" != "$KB_POD_NAME" ]; then
    export POSTGRES_REPLICATION_MODE=slave
    export POSTGRESQL_REPLICATION_USER=$POSTGRES_USER
    export POSTGRESQL_REPLICATION_PASSWORD=$POSTGRES_PASSWORD
    export POSTGRESQL_CLUSTER_APP_NAME=my-application
    export POSTGRESQL_MASTER_HOST=$KB_0_HOSTNAME
    export POSTGRESQL_MASTER_PORT_NUMBER="5432"
    export primary_conninfo="host=$KB_0_HOSTNAME port=$POSTGRESQL_PORT_NUMBER user=$PGUSER password=$PGPASSWORD application_name=$POSTGRESQL_CLUSTER_APP_NAME"
    # add permission to daemon user
    chmod a+w "$POSTGRESQL_VOLUME_DIR"
    # Ensure 'daemon' user exists when running as 'root'
    am_i_root && ensure_user_exists "$POSTGRESQL_DAEMON_USER" --group "$POSTGRESQL_DAEMON_GROUP"
    postgresql_slave_init_db
    postgresql_set_property "primary_conninfo" "$primary_conninfo" "$POSTGRESQL_CONF_FILE"
    touch "$POSTGRESQL_DATA_DIR"/standby.signal
  fi
fi
docker-entrypoint.sh --config-file=/kubeblocks/conf/postgresql.conf --hba_file=/kubeblocks/conf/pg_hba.conf