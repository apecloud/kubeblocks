#!/bin/bash
pvs=$(kubectl get pv | grep Released | awk '{print $1}')

for pv in $pvs; do
  kubectl delete pv $pv
done