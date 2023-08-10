#!/bin/bash
current_component_replicas=`cat /etc/annotations/component-replicas`
idx=${KB_POD_NAME##*-}
if [[ $idx -ge $current_component_replicas && $current_component_replicas -ne 0 ]]; then
   bin/bookkeeper shell bookieformat -nonInteractive -force -deleteCookie || true
fi