{{- $clusterName := $.cluster.metadata.name }}
{{- $namespace := $.cluster.metadata.namespace }}
{{- $orioledb_etcd_from_service_ref := fromJson "{}" }}
{{- if index $.component "serviceReferences" }}
  {{- range $i, $e := $.component.serviceReferences }}
    {{- if eq $i "etcdService" }}
      {{- $orioledb_etcd_from_service_ref = $e }}
      {{- break }}
    {{- end }}
  {{- end }}
{{- end }}
{{- $etcd_server := "" }}
{{- if $orioledb_etcd_from_service_ref }}
  {{- if and (index $orioledb_etcd_from_service_ref.spec "endpoint") (index $orioledb_etcd_from_service_ref.spec "port") }}
     {{- $etcd_server = printf "%s:%s" $orioledb_etcd_from_service_ref.spec.endpoint.value $orioledb_etcd_from_service_ref.spec.port.value }}
  {{- end }}
{{- end }}
export PATRONI_ETCD3_HOST={{ $etcd_server }}