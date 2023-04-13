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

{{/*
Expand the endpoint of the secret.
*/}}
{{- define "secret.endpoint" -}}
{{- if hasPrefix "cn-" .Values.secret.region }}
{{- printf "https://s3.%s.amazonaws.com.cn" .Values.secret.region }}
{{- else if .Values.secret.region }}
{{- printf "https://s3.%s.amazonaws.com" .Values.secret.region }}
{{- else }}
{{- default "https://s3.amazonaws.com" }}
{{- end }}
{{- end }}
