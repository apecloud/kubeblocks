#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- /* find nebula-metad component */}}
{{- $metads := "" }}
{{- $nebula_metad_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "nebula-metad" }}
        {{- $nebula_metad_component = $e }}
    {{- end }}
{{- end }}
{{- $replicas := $nebula_metad_component.replicas | int }}
{{- range $n, $e := until $replicas }}
    {{- $podFQDN := printf "%s-%s-%d.%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_metad_component.name $n $clusterName $nebula_metad_component.name $namespace }}
    {{- $metad := printf "%s:9559" $podFQDN }}
    {{- $metads = printf "%s,%s" $metads $metad }}
{{- end }}

exec /usr/local/nebula/bin/nebula-graphd --flagfile=/usr/local/nebula/etc/nebula-graphd.conf --meta_server_addrs={{$metads}} --local_ip=$KB_POD_FQDN".cluster.local" --ws_ip=$KB_POD_FQDN".cluster.local" --daemonize=false