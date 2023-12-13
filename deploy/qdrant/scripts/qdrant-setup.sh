#!/usr/bin/env bash

IDX=${KB_POD_NAME##*-}
HOSTNAME=$(eval echo \$KB_QDRANT_"${IDX}"_HOSTNAME)
BOOTSTRAP_HOSTNAME=$(eval echo \$KB_QDRANT_0_HOSTNAME)

if [ "$IDX" == "0" ]; then
  ./qdrant --uri "http://${HOSTNAME}:6335"
else
  echo "BOOTSTRAP_HOSTNAME: ${BOOTSTRAP_HOSTNAME}"
  until ./tools/curl http://${BOOTSTRAP_HOSTNAME}:6333/cluster; do
    echo "INFO: wait for bootstrap node starting..."
    sleep 1;
  done
  ./qdrant --bootstrap "http://${BOOTSTRAP_HOSTNAME}:6335" --uri "http://${HOSTNAME}:6335"
fi
