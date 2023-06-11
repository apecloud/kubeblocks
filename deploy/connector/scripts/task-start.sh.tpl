#!/bin/bash


if [ -z "$KB_CHANNEL_CONFIG" ]; then
  return 0
fi

# upset connector
curl -f -s -X PUT -H "Content-Type: application/json" http://localhost:8083/connectors/"${KB_CHANNEL_CONNECTOR_NAME}"/config -d "${KB_CHANNEL_CONFIG}"