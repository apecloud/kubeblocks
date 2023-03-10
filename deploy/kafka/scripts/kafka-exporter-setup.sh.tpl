#!/bin/bash
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find kafka-server component */}}
{{- $component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "kafka-server" }}
  {{- $component = $e }}
  {{- end }}
{{- end }}
{{- if not $component  }}
{{- /* find kafka-broker component */}}
  {{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "kafka-broker" }}
    {{- $component = $e }}
    {{- end }}
  {{- end }}
{{- end }}
{{- /* build --kafka.server= string */}}
{{- $replicas := $component.replicas | int }}
{{- $servers := "" }}
{{- range $i, $e := until $replicas }}
  {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc" $clusterName $component.name $i $clusterName $component.name $namespace }}
  {{- $server := printf "--kafka.server=%s:9092 \\\n" $podFQDN }}
  {{- $servers = printf "%s\t%s" $servers $server }}
{{- end }}
{{ $servers = trimSuffix " \\\n" $servers}}
exec kafka_exporter --web.listen-address=:9308 \
{{- if hasKey $component "tls" }}
  {{- if $component.tls }}
    --tls.enabled \
  {{- end }}
{{- end }}
{{ $servers }}