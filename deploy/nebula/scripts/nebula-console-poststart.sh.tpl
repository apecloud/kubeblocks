#!/bin/sh
set -ex
{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $graphd_port := 9669 }}
{{- $nebula_graphd_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
    {{- if eq $e.componentDefRef "nebula-graphd" }}
        {{- $nebula_graphd_component = $e }}
    {{- end }}
{{- end }}
{{- $graphd_svc := printf "%s-%s.%s.svc.cluster.local" $clusterName $nebula_graphd_component.name $namespace }}
echo "Waiting for graphd service {{ $graphd_svc }}  to be ready..."
until /usr/local/bin/nebula-console --addr {{ $graphd_svc }} --port {{$graphd_port}} --user root --password nebula -e "show spaces"; do sleep 2; done
touch  /tmp/nebula-storaged-hosts
echo ADD HOSTS \"$KB_POD_FQDN.cluster.local\":9779 > /tmp/nebula-storaged-hosts
exec /usr/local/bin/nebula-console --addr {{ $graphd_svc }} --port {{$graphd_port}} --user root --password nebula -f /tmp/nebula-storaged-hosts
rm /tmp/nebula-storaged-hosts
echo "Start Console succeeded!"