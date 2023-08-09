#!/bin/sh
while [ "$(curl -s -o /dev/null -w '%{http_code}' http://${brokerSVC}:80/status.html)" -ne "200" ]; do
  echo "pulsar cluster isn't initialized yet..."; sleep 1;
done