{{/*
Define redis cluster sentinel component.
*/}}
{{- define "redis-cluster.sentinel" }}
- name: redis-sentinel
  componentDefRef: redis-sentinel
  replicas: {{ .Values.sentinel.replicas }}
  resources:
    limits:
      cpu: {{ .Values.sentinel.cpu | quote }}
      memory:  {{ print .Values.sentinel.memory "Gi" | quote }}
    requests:
      cpu: {{ .Values.sentinel.cpu | quote }}
      memory:  {{ print .Values.sentinel.memory "Gi" | quote }}
  volumeClaimTemplates:
    - name: data
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: {{ print .Values.sentinel.storage "Gi" }}
{{- end }}


{{/*
Define redis cluster proxy component.
*/}}
{{- define "redis-cluster.proxy" }}
- name: redis-proxy
  componentDefRef: redis-proxy
  serviceAccountName: {{ include "kblib.serviceAccountName" . }}
  replicas: {{ .Values.proxy.replicas }}
  resources:
    limits:
      cpu: {{ .Values.proxy.cpu | quote }}
      memory: {{ print .Values.proxy.memory "Gi" | quote }}
    requests:
      cpu: {{ .Values.proxy.cpu | quote }}
      memory: {{ print .Values.proxy.memory "Gi" | quote }}
{{- end }}

{{/*
Define replica count.
standalone mode: 1
replication mode: 2
*/}}
{{- define "redis-cluster.replicaCount" }}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else if eq .Values.mode "replication" }}
replicas: {{ max .Values.replicas 2 }}
{{- end }}
{{- end }}