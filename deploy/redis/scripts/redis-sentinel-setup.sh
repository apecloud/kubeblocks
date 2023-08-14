#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find redis-sentinel component */}}
{{- $sentinel_component := fromJson "{}" }}
{{- $redis_component := fromJson "{}" }}
{{- $candidate_instance_index := 0 }}
{{- $primary_pod := "" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "redis-sentinel" }}
    {{- $sentinel_component = $e }}
  {{- else if eq $e.componentDefRef "redis" }}
    {{- $redis_component = $e }}
  {{- end }}
{{- end }}
{{- /* build primary pod message, because currently does not support cross-component acquisition of environment variables, the service of the redis master node is assembled here through specific rules  */}}
{{- $primary_pod = printf "%s-%s-%d.%s-%s-headless.%s.svc" $clusterName $redis_component.name $candidate_instance_index $clusterName $redis_component.name $namespace }}
{{- $sentinel_monitor := printf "%s-%s %s" $clusterName $redis_component.name $primary_pod }}
{{- /* build sentinel config */}}
echo "port 26379" > /etc/sentinel/redis-sentinel.conf
echo "sentinel resolve-hostnames yes" >> /etc/sentinel/redis-sentinel.conf
echo "sentinel announce-hostnames yes" >> /etc/sentinel/redis-sentinel.conf
echo "sentinel monitor {{ $sentinel_monitor }} 6379 2" >> /etc/sentinel/redis-sentinel.conf
echo "sentinel down-after-milliseconds {{ $clusterName }}-{{ $redis_component.name }} 5000" >> /etc/sentinel/redis-sentinel.conf
echo "sentinel failover-timeout {{ $clusterName }}-{{ $redis_component.name }} 60000" >> /etc/sentinel/redis-sentinel.conf
echo "sentinel parallel-syncs {{ $clusterName }}-{{ $redis_component.name }} 1" >> /etc/sentinel/redis-sentinel.conf
if [ ! -z "$REDIS_SENTINEL_PASSWORD" ]; then
  echo "sentinel auth-user {{ $clusterName }}-{{ $redis_component.name }} $REDIS_SENTINEL_USER" >> /etc/sentinel/redis-sentinel.conf
  echo "sentinel auth-pass {{ $clusterName }}-{{ $redis_component.name }} $REDIS_SENTINEL_PASSWORD" >> /etc/sentinel/redis-sentinel.conf
fi
if [ ! -z "$SENTINEL_PASSWORD" ]; then
  echo "sentinel sentinel-user $SENTINEL_USER" >> /etc/sentinel/redis-sentinel.conf
  echo "sentinel sentinel-pass $SENTINEL_PASSWORD" >> /etc/sentinel/redis-sentinel.conf
fi
{{- /* $primary_svc := printf "%s-%s.%s.svc" $clusterName $redis_component.name $namespace */}}