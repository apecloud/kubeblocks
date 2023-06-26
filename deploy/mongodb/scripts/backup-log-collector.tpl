#!/bin/sh
type=$1
if [ "${type}" = "post" ]; then
  stopTime=$(mongosh --eval 'db.isMaster().lastWrite.lastWriteDate.getTime()/1000' --quiet)
  stopTime=$(date -d "@${stopTime}" -u '+%Y-%m-%dT%H:%M:%SZ')
  printf "{\"stopTime\":\"${stopTime}\"}"
else
  startTime=$(mongosh --eval 'db.isMaster().lastWrite.lastWriteDate.getTime()/1000' --quiet)
  startTime=$(date -d "@${startTime}" -u '+%Y-%m-%dT%H:%M:%SZ')
  printf "{\"startTime\":\"${startTime}\"}"
fi
