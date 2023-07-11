{{/*
Define the cluster componnets with proxy.
The proxy cpu cores is 1/6 of the cluster total cpu cores.
*/}}
{{- define "apecloud-mysql-cluster.proxyComponents" }}
{{- $replicas := (include "apecloud-mysql-cluster.replicas" .) }}
{{- $proxyCPU := (int (ceil (div (mul $replicas .Values.cpu) 6))) }}
{{- if lt $proxyCPU 2 }}
{{- $proxyCPU = 2 }}
{{- end }}
{{- if gt $proxyCPU 64 }}
{{- $proxyCPU = 64 }}
{{- end }}
{{- if eq (mod $proxyCPU 2) 1 }}
{{- $proxyCPU = add $proxyCPU 1 }}
{{- end }}
- name: vtcontroller
  componentDefRef: vtcontroller # ref clusterdefinition componentDefs.name
  volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  replicas: 1
  resources:
    limits:
      cpu: 500m
      memory: 128Mi
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
Define replicas.
standalone mode: 1
raftGroup mode: max(replicas, 3)
*/}}
{{- define "apecloud-mysql-cluster.replicas" }}
{{- if eq .Values.mode "standalone" }}
{{- 1 }}
{{- else if eq .Values.mode "raftGroup" }}
{{- max .Values.replicas 3 }}
{{- end }}
{{- end -}}