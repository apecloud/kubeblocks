#!/bin/bash
set -x
mkdir -p ${journalDirectories}/current && mkdir -p ${ledgerDirectories}/current
journalRes=`ls -A ${journalDirectories}/current`
ledgerRes=`ls -A ${ledgerDirectories}/current`
if [[ -z $journalRes && -z $ledgerRes ]]; then
   host_ip_port="${KB_POD_FQDN}${cluster_domain}:3181"
   zNode="${zkLedgersRootPath}/cookies/${host_ip_port}"
   # if current dir are empty but bookieId exists in zookeeper, delete it
   if zkURL=${zkServers} python3 /kb-scripts/zookeeper.py get ${zNode}; then
     echo "Warning: exist redundant bookieID ${zNode}"
     zkURL=${zkServers} python3 /kb-scripts/zookeeper.py delete ${zNode};
   fi
fi
python3 /kb-scripts/merge_pulsar_config.py conf/bookkeeper.conf /opt/pulsar/conf/bookkeeper.conf;
bin/apply-config-from-env.py conf/bookkeeper.conf;
OPTS="${OPTS} -Dlog4j2.formatMsgNoLookups=true" exec bin/pulsar bookie;