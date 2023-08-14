#!/bin/bash
set -o errexit
set -e

# usage: retry <command>
# e.g. retry pg_isready -U postgres -h $primary_fqdn -p 5432
function retry {
  local max_attempts=10
  local attempt=1
  until "$@" || [ $attempt -eq $max_attempts ]; do
    echo "Command '$*' failed. Attempt $attempt of $max_attempts. Retrying in 5 seconds..."
    attempt=$((attempt + 1))
    sleep 5
  done
  if [ $attempt -eq $max_attempts ]; then
    echo "Command '$*' failed after $max_attempts attempts. Exiting..."
    exit 1
  fi
}

if [ -f /kb-podinfo/primary-pod ]; then
  # Waiting for primary pod information from the DownwardAPI annotation to be available, with a maximum of 5 attempts
  attempt=1
  max_attempts=10
  while [ $attempt -le $max_attempts ] && [ -z "$(cat /kb-podinfo/primary-pod)" ]; do
    sleep 3
    attempt=$((attempt + 1))
  done
  primary=$(cat /kb-podinfo/primary-pod)
  echo "DownwardAPI get primary=$primary" >> /home/postgres/pgdata/.kb_set_up.log
  echo "KB_POD_NAME=$KB_POD_NAME" >> /home/postgres/pgdata/.kb_set_up.log
else
   echo "DownwardAPI get /kb-podinfo/primary-pod is empty" >> /home/postgres/pgdata/.kb_set_up.log
fi

if  [ ! -z "$primary" ] && [ "$primary" != "$KB_POD_NAME" ]; then
    primary_fqdn="$primary.$KB_CLUSTER_NAME-$KB_COMP_NAME-headless.$KB_NAMESPACE.svc"
    echo "primary_fqdn=$primary_fqdn" >> /home/postgres/pgdata/.kb_set_up.log
    # waiting for the primary to be ready, if the wait time exceeds the maximum number of retries, then the script will fail and exit.
    retry pg_isready -U "postgres" -h $primary_fqdn -p 5432
fi

if [ -f ${RESTORE_DATA_DIR}/kb_restore.signal ]; then
    chown -R postgres ${RESTORE_DATA_DIR}
fi
python3 /kb-scripts/generate_patroni_yaml.py tmp_patroni.yaml
export SPILO_CONFIGURATION=$(cat tmp_patroni.yaml)
exec /launch.sh init
