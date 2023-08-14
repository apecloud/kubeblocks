#!/bin/sh
if [ "$KB_PROXY_ENABLED" != "on" ]; then
  /bin/agamotto --config=/opt/agamotto/agamotto-config.yaml
else
  /bin/agamotto --config=/opt/agamotto/agamotto-config-with-proxy.yaml
fi 