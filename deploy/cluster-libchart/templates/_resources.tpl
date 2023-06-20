{{/*
Define component resources, including cpu, memory and storage
*/}}
{{- define "cluster-libchart.componentResources" }}
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
          storage: {{ print .Values.storageSize "Gi" }}
{{- end }}