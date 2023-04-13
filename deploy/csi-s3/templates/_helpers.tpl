{{/*
Expand the mountOptions of the storageClass.
*/}}
{{- define "storageClass.mountOptions" -}}
{{- if .Values.secret.region }}
{{- printf "%s --region %s" .Values.storageClass.mountOptions .Values.secret.region }}
{{- else }}
{{- .Values.storageClass.mountOptions }}
{{- end }}
{{- end }}