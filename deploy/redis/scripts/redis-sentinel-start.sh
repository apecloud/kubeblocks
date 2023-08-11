#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find redis component */}}
{{- $redis_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "redis" }}
  {{- $redis_component = $e }}
  {{- end }}
{{- end }}
{{- /* build redis engine service */}}
{{- $primary_svc := printf "%s-%s.%s.svc" $clusterName $redis_component.name $namespace }}
echo "Waiting for redis service {{ $primary_svc }} to be ready..."
if [ ! -z "$REDIS_DEFAULT_PASSWORD" ]; then
  until redis-cli -h {{ $primary_svc }} -p 6379 -a $REDIS_DEFAULT_PASSWORD ping; do sleep 1; done
else
  until redis-cli -h {{ $primary_svc }} -p 6379 ping; do sleep 1; done
fi
echo "redis service ready, Starting sentinel..."
echo "sentinel announce-ip $KB_POD_FQDN" >> /etc/sentinel/redis-sentinel.conf
exec redis-server /etc/sentinel/redis-sentinel.conf --sentinel
echo "Start sentinel succeeded!"