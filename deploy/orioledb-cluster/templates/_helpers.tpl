{{/*
Define replica count.
standalone mode: 1
replication mode: 2
*/}}
{{- define "orioledb-cluster.replicaCount" }}
{{- if eq .Values.mode "standalone" }}
replicas: 1
{{- else if eq .Values.mode "replication" }}
replicas: {{ max .Values.replicas 2 }}
{{- end }}
{{- end }}