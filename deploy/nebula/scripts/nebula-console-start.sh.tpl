#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $graphd_port := 9669 }}
{{- $storaged_port := 9779 }}
{{- $nebula_storaged_component := fromJson "{}" }}
{{- $nebula_graphd_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "nebula-graphd" }}
        {{- $nebula_graphd_component = $e }}
    {{- else if eq $e.componentDefRef "nebula-storaged" }}
        {{- $nebula_storaged_component = $e }}
    {{- end }}
{{- end }}
{{- $graphd_svc := printf "%s-%s.%s.svc.cluster.local" $clusterName $nebula_graphd_component.name $namespace }}
{{- $storaged_svc_suffix := printf "%s-%s-headless.%s.svc.cluster.local" $clusterName $nebula_storaged_component.name $namespace }}

echo "Waiting for graphd service {{ $graphd_svc }} to be ready..."
until /usr/local/bin/nebula-console --addr {{ $graphd_svc }} --port {{$graphd_port}} --user root --password nebula -e "show spaces"; do sleep 1; done
echo "graphd service is ready, add storaged address..."
id=0
while [ $id -lt {{$nebula_storaged_component.replicas}} ]
do
  /usr/local/bin/nebula-console --addr {{ $graphd_svc }} --port {{$graphd_port}} --user root --password nebula -e "ADD HOSTS \"{{ $clusterName }}-{{ $nebula_storaged_component.name }}-$id.{{ $storaged_svc_suffix }}\":{{ $storaged_port }};"
  id=$(( $id + 1 ))
done
echo "Start Console succeeded!"