{{/*
Define component resources, including cpu, memory and storage
*/}}
{{- define "kblib.componentResources" }}
{{- $requestCPU := (float64 .Values.cpu) }}
{{- $requestMemory := (float64 .Values.memory) }}
{{- if .Values.requests }}
{{- if and .Values.requests.cpu (lt (float64 .Values.requests.cpu) $requestCPU) }}
{{- $requestCPU = .Values.requests.cpu }}
{{- end }}
{{- if and .Values.requests.memory (lt (float64 .Values.requests.memory) $requestMemory) }}
{{- $requestMemory = .Values.requests.memory }}
{{- end }}
{{- end }}
resources:
  limits:
    cpu: {{ .Values.cpu | quote }}
    memory: {{ print .Values.memory "Gi" | quote }}
  requests:
    cpu: {{ $requestCPU | quote }}
    memory: {{ print $requestMemory "Gi" | quote }}
volumeClaimTemplates:
  - name: data # ref clusterDefinition components.containers.volumeMounts.name
    spec:
      accessModes:
        - ReadWriteOnce
      resources:
        requests:
          storage: {{ print .Values.storage "Gi" }}
{{- end }}