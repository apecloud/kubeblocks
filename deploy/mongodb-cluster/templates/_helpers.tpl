{{/*
Define replica count.
standalone mode: 1
replicaset mode: 3
*/}}
{{- define "mongodb-cluster.replicaCount" }}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else if eq .Values.mode "replicaset" }}
replicas: {{ max .Values.replicas 3 }}
{{- end }}
{{- end }}