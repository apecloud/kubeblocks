{{/*
Define component resources, including cpu, memory and storage
*/}}
{{- define "cluster-libchart.componentResources" }}
{{- $cpu := (float64 .Values.cpu) }}
{{- $memory := (float64 .Values.memory) }}
{{- $storage := (float64 .Values.storage) }}
{{- if or (lt $cpu 0.5) (gt $cpu 64.0) }}
{{- fail (print "cpu must be between 0.5 and 64, got " $cpu) }}
{{- end }}
{{- if or (lt $memory 0.5) (gt $memory 1000.0) }}
{{- fail (print "memory must be between 0.5 and 1000, got " $memory) }}
{{- end }}
{{- if or (lt $storage 10) (gt $storage 10000.0) }}
{{- fail (print "storage must be between 10 and 1000, got " $storage) }}
{{- end }}
resources:
  limits:
    cpu: {{ .Values.cpu | quote }}
    memory: {{ print .Values.memory "Gi" | quote }}
  requests:
    cpu: {{ .Values.cpu | quote }}
    memory: {{ print .Values.memory "Gi" | quote }}
volumeClaimTemplates:
  - name: data # ref clusterDefinition components.containers.volumeMounts.name
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: {{ print .Values.storage "Gi" }}
{{- end }}