{{/*
Define the cluster componnets with proxy.
The proxy cpu cores is 1/6 of the cluster total cpu cores and is multiple of 0.5.
The minimum proxy cpu cores is 0.5 and the maximum cpu cores is 64.
*/}}
{{- define "apecloud-mysql-cluster.proxyComponents" }}
{{- $replicas := (include "apecloud-mysql-cluster.replicas" .) }}
{{- $proxyCPU := divf (mulf $replicas .Values.cpu) 6.0 }}
{{- $proxyCPU = divf $proxyCPU 0.5 | ceil | mulf 0.5 }}
{{- if lt $proxyCPU 0.5 }}
{{- $proxyCPU = 0.5 }}
{{- else if gt $proxyCPU 64.0 }}
{{- $proxyCPU = 64 }}
{{- end }}
- name: vtcontroller
  componentDefRef: vtcontroller # ref clusterdefinition componentDefs.name
  enabledLogs:
    - error
    - warning
    - info
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
  enabledLogs:
    - error
    - warning
    - info
    - queryLog
  resources:
    requests:
      cpu: {{ $proxyCPU | quote }}
    limits:
      cpu: {{ $proxyCPU | quote }}
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