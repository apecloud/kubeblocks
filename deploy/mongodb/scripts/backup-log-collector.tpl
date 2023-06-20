#!/bin/sh
stopTime=$(mongosh --eval 'db.isMaster().lastWrite.lastWriteDate.getTime()/1000' --quiet)
stopTime=$(date -d "@${stopTime}" -u '+%Y-%m-%dT%H:%M:%SZ')
printf "{\"stopTime\":\"${stopTime}\"}"