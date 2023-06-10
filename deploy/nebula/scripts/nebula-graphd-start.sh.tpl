#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find nebula-metad component */}}
{{- $metad_pod := "" }}
{{- $metad_port := 9559 }}
{{- $graphd_port := 9669 }}
{{- $nebula_metad_component := fromJson "{}" }}
{{- $nebula_graphd_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "nebula-metad" }}
        {{- $nebula_metad_component = $e }}
    {{- else if eq $e.componentDefRef "nebula-graphd" }}
        {{- $nebula_graphd_component = $e }}
    {{- end }}
{{- end }}
{{- $metad_pod = printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_metad_component.name 0 $clusterName $nebula_metad_component.name $namespace }}
{{- $svc_suffix := printf "%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_graphd_component.name $namespace }}
exec /usr/local/nebula/bin/nebula-graphd --flagfile=/usr/local/nebula/etc/nebula-graphd.conf --meta_server_addrs={{$metad_pod}}:{{$metad_port}} --local_ip=$(hostname).{{$svc_suffix}} --ws_ip=$(hostname).{{$svc_suffix}} --daemonize=false