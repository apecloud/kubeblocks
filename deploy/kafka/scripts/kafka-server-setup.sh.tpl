#!/bin/bash
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{/* build KAFKA_CFG_CONTROLLER_QUORUM_VOTERS value string */}}
{{- $replicas := $.component.replicas | int }}
{{- $voters := "" }}
{{- range $i, $e := until $replicas }}
  {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.$s.svc" $clusterName $.component.name $i $clusterName $.component.name $namespace  }}
  {{- $voter := printf "%d@%s:9093" $i $podFQDN }}
  {{- $voters = printf "%s,%s" $voters $voter }}
{{- end }}
{{- trimPrefix "," $voters }}

ID="${KB_POD_NAME#"${KB_CLUSTER_COMP_NAME}-"}"
export KAFKA_CFG_BROKER_ID="$((ID + 0))"
if [[ "$KAFKA_ENABLE_KRAFT" == "yes" ]]; then
    export KAFKA_CFG_CONTROLLER_QUORUM_VOTERS={{ $voters }}
fi

exec /entrypoint.sh /run.sh