#!/bin/sh
set -x
echo "INFO: wait for zookeeper ready..."
zkDomain="${zookeeperServers%%:*}"
until zkURL=${zookeeperServers} python3 /kb-scripts/zookeeper.py get /; do
  sleep 1;
done;
idx=${KB_POD_NAME##*-}
if [ $idx -ne 0 ]; then
  # if not the first pod, do it
  until zkURL=${zookeeperServers} python3 /kb-scripts/zookeeper.py get /admin/clusters/${clusterName}; do
    echo "INFO: wait for init the meta cluster..."
    sleep 1;
  done
  echo "INFO: cluster already initialized" && exit 0
fi
# if the pod is the first pod, do it
if zkURL=${zookeeperServers} python3 /kb-scripts/zookeeper.py get /admin/clusters/${clusterName}; then
  echo "INFO: cluster already initialized" && exit 0
fi
echo "INFO: init cluster metadata for cluster: ${clusterName}"
bin/pulsar initialize-cluster-metadata \
--cluster ${clusterName} \
--zookeeper ${zookeeperServers} \
--configuration-store ${zookeeperServers} \
--web-service-url ${webServiceUrl} \
--broker-service-url ${brokerServiceUrl}

(curl -sf -XPOST http://127.0.0.1:15020/quitquitquit || true) && exit 0