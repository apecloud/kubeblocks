{{/*
Define the cluster componnets with proxy
TODO: calculate the proxy resoruces based on the mysql resources
*/}}
{{- define "apecloud-mysql-cluster.proxyComponents" }}
- name: etcd
  componentDefRef: etcd # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtctld
  componentDefRef: vtctld # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtconsensus
  componentDefRef: vtconsensus # ref clusterdefinition componentDefs.name
  replicas: 1
- name: vtgate
  componentDefRef: vtgate # ref clusterdefinition componentDefs.name
  replicas: 1
{{- end }}

{{/*
Define replica count.
standalone mode: 1
replication mode: 2
raftGroup mode: 3 or more
*/}}
{{- define "apecloud-mysql-cluster.replicaCount" -}}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else if eq .Values.mode "replication" }}
replicas: 2
{{- else }}
replicas: {{ max .Values.replicas 3 }}
{{- end }}
{{- end -}}