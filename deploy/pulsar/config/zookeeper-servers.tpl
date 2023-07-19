{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $zk_servers := "" }}
{{- $zk_server_i := "" }}
{{- $replicas := $pulsar_zk_component.replicas | int }}
{{- range $i, $e := until $replicas }}
  {{- $zk_server_i = printf "%s-%s-%d\n" $clusterName $pulsar_zk_component.name $i }}
  {{- if ne $zk_servers "" }}
    {{- $zk_servers = printf "%s,%s" $zk_servers $zk_server_i }}
  {{- else }}
    {{- $zk_servers = printf "%s" $zk_server_i }}
  {{- end }}
{{- end }}
{{- $zk_servers }}