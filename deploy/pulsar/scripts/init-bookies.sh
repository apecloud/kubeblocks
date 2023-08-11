#!/bin/bash
echo "waiting zookeeper ready..."
zkDomain="${zkServers%%:*}"
until echo ruok | nc -q 1 ${zkDomain} 2181 | grep imok; do
  sleep 1;
done;
echo "zk is ready, start to config bookkeeper..."
python3 /kb-scripts/merge_pulsar_config.py conf/bookkeeper.conf /opt/pulsar/conf/bookkeeper.conf;
bin/apply-config-from-env.py conf/bookkeeper.conf;
if bin/bookkeeper shell whatisinstanceid; then
  echo "bookkeeper cluster already initialized";
else
  echo "bookkeeper init new cluster."
  bin/bookkeeper shell initnewcluster;
fi