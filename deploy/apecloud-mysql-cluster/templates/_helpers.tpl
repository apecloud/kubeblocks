{{/*
Define the cluster componnets with proxy.
The proxy cpu cores is 1/6 of the cluster total cpu cores.
*/}}
{{- define "apecloud-mysql-cluster.proxyComponents" }}
{{- $proxyCPU := (int (ceil (div (mul .Values.replicas .Values.cpu) 6))) }}
{{- if lt $proxyCPU 2 }}
{{- $proxyCPU = 2 }}
{{- end }}
{{- if gt $proxyCPU 64 }}
{{- $proxyCPU = 64 }}
{{- end }}
{{- if eq (mod $proxyCPU 2) 1 }}
{{- $proxyCPU = add $proxyCPU 1 }}
{{- end }}
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
  resources:
    requests:
      cpu: {{ $proxyCPU }}
    limits:
      cpu: {{ $proxyCPU }}
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
replicas: {{ max .Values.replicas 2 }}
{{- else }}
replicas: {{ max .Values.replicas 3 }}
{{- end }}
{{- end -}}