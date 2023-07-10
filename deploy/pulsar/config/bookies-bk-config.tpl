{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $zk_server := "" }}
{{- $zk_server = printf "%s-%s" $clusterName $pulsar_zk_component }}
{{ $zk_server }}:2181