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
  timeout 300 sh -c 'until redis-cli -h {{ $primary_svc }} -p 6379 -a $REDIS_DEFAULT_PASSWORD ping; do sleep 2; done'
else
  timeout 300 sh -c 'until redis-cli -h {{ $primary_svc }} -p 6379 ping; do sleep 1; done'
fi
if [ $? -ne 0 ]; then
  echo "Redis service is not ready, exiting..."
  exit 1
fi
echo "Redis service ready, Starting sentinel..."
echo "sentinel announce-ip $KB_POD_FQDN" >> /etc/sentinel/redis-sentinel.conf
exec redis-server /etc/sentinel/redis-sentinel.conf --sentinel
echo "Start sentinel succeeded!"