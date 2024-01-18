{{/*
Create the name of the ServiceAccount for worker pods.
*/}}
{{- define "dataprotection.workerSAName" -}}
{{- if .Values.dataProtection.worker.serviceAccount.name }}
{{- .Values.dataProtection.worker.serviceAccount.name }}
{{- else }}
{{- include "kubeblocks.fullname" . }}-dataprotection-worker
{{- end }}
{{- end }}

{{/*
Create the name of the ClusterRole for worker pods.
*/}}
{{- define "dataprotection.workerClusterRoleName" -}}
{{- include "kubeblocks.fullname" . }}-dataprotection-worker-role
{{- end }}

