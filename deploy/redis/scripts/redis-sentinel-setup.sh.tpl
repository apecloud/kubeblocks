#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find redis-sentinel component */}}
{{- $sentinel_component := fromJson "{}" }}
{{- $redis_component := fromJson "{}" }}
{{- $primary_index := 0 }}
{{- $primary_pod := "" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "redis-sentinel" }}
    {{- $sentinel_component = $e }}
  {{- else if eq $e.componentDefRef "redis" }}
    {{- $redis_component = $e }}
    {{- if index $e "primaryIndex" }}
        {{- if ne ($e.primaryIndex | int) 0 }}
          {{- $primary_index = ($e.primaryIndex | int) }}
        {{- end }}
    {{- end }}
  {{- end }}
{{- end }}
{{- /* build primary pod message, because currently does not support cross-component acquisition of environment variables, the service of the redis master node is assembled here through specific rules  */}}
{{- $primary_pod = printf "%s-%s-%d.%s-%s-headless.%s.svc" $clusterName $redis_component.name $primary_index $clusterName $redis_component.name $namespace }}
{{- $sentinel_monitor := printf "%s-%s %s" $clusterName $redis_component.name $primary_pod }}
cat>/etc/sentinel/redis-sentinel.conf<<EOF
port 26379
sentinel resolve-hostnames yes
sentinel announce-hostnames yes
sentinel monitor {{ $sentinel_monitor }} 6379 2
sentinel down-after-milliseconds {{ $clusterName }}-{{ $redis_component.name }} 5000
sentinel failover-timeout {{ $clusterName }}-{{ $redis_component.name }} 60000
sentinel parallel-syncs {{ $clusterName }}-{{ $redis_component.name }} 1
{{- /* $primary_svc := printf "%s-%s.%s.svc" $clusterName $redis_component.name $namespace */}}
EOF