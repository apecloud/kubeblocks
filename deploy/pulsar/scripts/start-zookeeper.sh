#!/bin/bash
set -e

bin/apply-config-from-env.py conf/zookeeper.conf;
bin/generate-zookeeper-config.sh conf/zookeeper.conf; exec bin/pulsar zookeeper;