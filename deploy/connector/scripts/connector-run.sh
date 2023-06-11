#!/usr/bin/env bash
#
# Copyright 2016 Confluent Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# add by ApeCloud for init jar
if [ "$KB_CHANNEL_TYPE" ];then
  kafkaHost=$KB_CHANNEL_SOURCE_HOSTNAME
  kafkaPort=$KB_CHANNEL_SOURCE_PORT
  if [ "sink" = "$KB_CHANNEL_TYPE" ];then
    kafkaHost=$KB_CHANNEL_SINK_HOSTNAME
    kafkaPort=$KB_CHANNEL_SINK_PORT
  fi
  if [ "$kafkaHost" ] && [ "$kafkaPort" ];then
    export CONNECT_BOOTSTRAP_SERVERS="$kafkaHost:$kafkaPort"
    echo "export CONNECT_BOOTSTRAP_SERVERS=$CONNECT_BOOTSTRAP_SERVERS"
    export CONNECT_CONFIG_STORAGE_TOPIC="${KB_CHANNEL_TOPOLOGY_NAME}_${KB_CHANNEL_NAME}_${KB_CHANNEL_TYPE}_configs"
    echo "export CONNECT_CONFIG_STORAGE_TOPIC=$CONNECT_CONFIG_STORAGE_TOPIC"
    export CONNECT_OFFSET_STORAGE_TOPIC="${KB_CHANNEL_TOPOLOGY_NAME}_${KB_CHANNEL_NAME}_${KB_CHANNEL_TYPE}_offsets"
    echo "export CONNECT_OFFSET_STORAGE_TOPIC=$CONNECT_OFFSET_STORAGE_TOPIC"
    export CONNECT_STATUS_STORAGE_TOPIC="${KB_CHANNEL_TOPOLOGY_NAME}_${KB_CHANNEL_NAME}_${KB_CHANNEL_TYPE}_status"
    echo "export CONNECT_STATUS_STORAGE_TOPIC=$CONNECT_STATUS_STORAGE_TOPIC"
  fi
fi

# finish add

. /etc/confluent/docker/bash-config

. /etc/confluent/docker/mesos-setup.sh
. /etc/confluent/docker/apply-mesos-overrides

echo "===> User"
id

echo "===> Configuring ..."
/etc/confluent/docker/configure

echo "===> Running preflight checks ... "
/etc/confluent/docker/ensure

# add by ApeCloud for init jar
confluent-hub install debezium/debezium-connector-mysql:2.1.4 --no-prompt
confluent-hub install debezium/debezium-connector-postgresql:2.2.1 --no-prompt
confluent-hub install confluentinc/kafka-connect-jdbc:10.7.0 --no-prompt
cd & \
cp /usr/share/confluent-hub-components/debezium-debezium-connector-mysql/lib/mysql-connector-j-8.0.32.jar /etc/kafka-connect/jars & \
cp /usr/share/confluent-hub-components/debezium-debezium-connector-postgresql/lib/postgresql-42.5.1.jar /etc/kafka-connect/jars

echo "===> Launching ... "
/etc/confluent/docker/launch