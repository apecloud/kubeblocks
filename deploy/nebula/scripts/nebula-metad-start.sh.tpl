#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find nebula-metad component */}}
{{- $metad_pod := "" }}
{{- $metad_port := 9559 }}
{{- $nebula_metad_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "nebula-metad" }}
        {{- $nebula_metad_component = $e }}
    {{- end }}
{{- end }}
{{- $metad_pod = printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_metad_component.name 0 $clusterName $nebula_metad_component.name $namespace }}
{{- $svc_suffix := printf "%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_metad_component.name $namespace }}
exec /usr/local/nebula/bin/nebula-metad --flagfile=/usr/local/nebula/etc/nebula-metad.conf --meta_server_addrs={{ $metad_pod }}:9559 --local_ip=$(hostname).{{$svc_suffix}} --ws_ip=$(hostname).{{$svc_suffix}}  --daemonize=false

