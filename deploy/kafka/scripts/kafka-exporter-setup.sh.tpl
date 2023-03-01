#!/bin/bash
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{/* find "kafka-server" component */}}
{{- $component := nil }}
{{- range $i, $e := until $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "kafka-server" }}
  {{- $component = $e }}
  {{- end }}
{{- end }}
{{- if not $component  }}
  {{- range $i, $e := until $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "kafka-broker" }}
    {{- $component = $e }}
    {{- end }}
  {{- end }}
{{- end }}

{{/* build --kafka.server= string */}}
{{- $replicas := $component.replicas | int }}
{{- $servers := "" }}
{{- range $i, $e := until $replicas }}
  {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.$s.svc" $clusterName $.component.name $i $clusterName $.component.name $namespace  }}
  {{- $server := printf "--kafka.server=%d@%s:9092" $i $podFQDN }}
  {{- $servers = printf "%s %s" $servers $server }}
{{- end }}

exec kafka_exporter {{ $servers }} --web.listen-address=:9308