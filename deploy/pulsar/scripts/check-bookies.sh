#!/bin/bash
set -x
bin/apply-config-from-env.py conf/bookkeeper.conf
until bin/bookkeeper shell whatisinstanceid; do
  sleep 3;
done;
sysctl -w net.ipv4.tcp_keepalive_time=1 && sysctl -w net.ipv4.tcp_keepalive_intvl=11 && sysctl -w net.ipv4.tcp_keepalive_probes=3