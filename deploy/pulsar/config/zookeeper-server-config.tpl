{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $pulsar_zk_component := fromJson "{}" }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef "zookeeper" }}
    {{- $pulsar_zk_component = $e }}
  {{- end }}
{{- end }}
{{- $replicas := $pulsar_zk_component.replicas | int }}
{{- range $i, $e := until $replicas }}
  {{- printf "server.%d=%s-%s-%d.%s-%s-headless.%s.svc:2888:3888:participant;0.0.0.0:2181\n" (add1 $i) $clusterName $pulsar_zk_component.name $i $clusterName $pulsar_zk_component.name $namespace }}
{{- end }}