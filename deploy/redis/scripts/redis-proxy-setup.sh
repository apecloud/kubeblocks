#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find redis-proxy component */}}
{{- $proxy_component := fromJson "{}" }}
{{- $redis_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "redis-proxy" }}
    {{- $proxy_component = $e }}
  {{- else if eq $e.componentDefRef "redis" }}
    {{- $redis_component = $e }}
  {{- end }}
{{- end }}
{{- /* build proxy config */}}
echo "alpha:" > /etc/proxy/nutcracker.conf
echo "  listen: 127.0.0.1:22121" >> /etc/proxy/nutcracker.conf
echo "  hash: fnv1a_64" >> /etc/proxy/nutcracker.conf
echo "  distribution: ketama" >> /etc/proxy/nutcracker.conf
echo "  auto_eject_hosts: true" >> /etc/proxy/nutcracker.conf
echo "  redis: true" >> /etc/proxy/nutcracker.conf
echo "  server_retry_timeout: 2000" >> /etc/proxy/nutcracker.conf
echo "  server_failure_limit: 1" >> /etc/proxy/nutcracker.conf
echo "  servers:" >> /etc/proxy/nutcracker.conf
echo "    - {{ $clusterName }}-{{ $redis_component.name }}.{{ $namespace }}.svc:6379:1 {{ $clusterName }}" >> /etc/proxy/nutcracker.conf