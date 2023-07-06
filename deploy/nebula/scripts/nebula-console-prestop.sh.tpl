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
idx=${KB_POD_NAME##*-}
current_component_replicas=`cat /etc/annotations/component-replicas`
if [ ! $idx -lt $current_component_replicas ] && [ $current_component_replicas -ne 0 ]; then
  storagedhost=$(echo DROP HOSTS \"$KB_POD_FQDN.cluster.local\":9779)
  touch /tmp/nebula-storaged-hosts
  echo DROP HOSTS \"$KB_POD_FQDN.cluster.local\":9779 > /tmp/nebula-storaged-hosts
  exec /usr/local/bin/nebula-console --addr {{ $graphd_svc }} --port {{$graphd_port}} --user root --password nebula -f /tmp/nebula-storaged-hosts
  rm /tmp/nebula-storaged-hosts
fi