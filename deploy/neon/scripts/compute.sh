#!/bin/sh
  set -ex

  SPEC_FILE_ORG=spec.prep.DOCKER.json
  SPEC_FILE=/tmp/spec.json
  PG_VERSION=14

  echo "Waiting pageserver become ready."
  while ! nc -z $PAGESERVER 6400; do
       sleep 1;
  done
  echo "Page server is ready."

  echo "Create a tenant and timeline"
  if [ -z "$TENANT" ]; then
  PARAMS=(
       -sb
       -X POST
       -H "Content-Type: application/json"
       -d "{}"
       http://${PAGESERVER}:9898/v1/tenant/
  )
  tenant_id=$(curl "${PARAMS[@]}" | sed 's/"//g')
  else
     tenant_id=$TENANT
  fi


  if [ -z "$TIMELINE" ]; then
  PARAMS=(
       -sb
       -X POST
       -H "Content-Type: application/json"
       -d "{\"tenant_id\":\"${tenant_id}\", \"pg_version\": ${PG_VERSION}}"
       "http://${PAGESERVER}:9898/v1/tenant/${tenant_id}/timeline/"
  )
  result=$(curl "${PARAMS[@]}")
  echo $result | jq .

  echo "Overwrite tenant id and timeline id in spec file"
  tenant_id=$(echo ${result} | jq -r .tenant_id)
  timeline_id=$(echo ${result} | jq -r .timeline_id)

  else

  #If not empty CREATE_BRANCH
  #we create branch with given ancestor_timeline_id as TIMELINE

  if [ ! -z "$CREATE_BRANCH" ]; then

  PARAMS=(
       -sb
       -X POST
       -H "Content-Type: application/json"
       -d "{\"tenant_id\":\"${tenant_id}\", \"pg_version\": ${PG_VERSION}, \"ancestor_timeline_id\":\"${TIMELINE}\"}"
       "http://${PAGESERVER}:9898/v1/tenant/${tenant_id}/timeline/"
  )

  result=$(curl "${PARAMS[@]}")
  echo $result | jq .

  echo "Overwrite tenant id and timeline id in spec file"
  tenant_id=$(echo ${result} | jq -r .tenant_id)
  timeline_id=$(echo ${result} | jq -r .timeline_id)

  else
      timeline_id=$TIMELINE
  fi #end if CREATE_BRANCH

  fi

  sed "s/TENANT_ID/${tenant_id}/" ${SPEC_FILE_ORG} > ${SPEC_FILE}
  sed -i "s/TIMELINE_ID/${timeline_id}/" ${SPEC_FILE}
  sed -i "s/PAGESERVER_SPEC/${PAGESERVER}/" ${SPEC_FILE}
  sed -i "s/SAFEKEEPERS_SPEC/${SAFEKEEPERS}/" ${SPEC_FILE}

  cat ${SPEC_FILE}

  echo "Start compute node"
  whoami
  echo $PWD
  ls -lah /data

  mkdir /data/pgdata
  /opt/neondatabase-neon/target/release/compute_ctl --pgdata /data/pgdata \
       -C "postgresql://cloud_admin@localhost:55432/postgres"  \
       -b /opt/neondatabase-neon/pg_install/v14/bin/postgres   \
       -S ${SPEC_FILE}
