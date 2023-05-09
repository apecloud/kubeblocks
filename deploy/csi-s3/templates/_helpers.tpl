{{/*
Expand the mountOptions of the storageClass.
*/}}
{{- define "storageClass.mountOptions" -}}
{{- if eq .Values.storageClass.mounter "geesefs" }}
  {{- if hasSuffix ".aliyuncs.com" .Values.secret.endpoint }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 --subdomain %s" .Values.storageClass.mountOptions }}
  {{- else if .Values.secret.region }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 --region %s %s" .Values.storageClass.mountOptions .Values.secret.region }}
  {{- else }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 %s" .Values.storageClass.mountOptions }}
  {{- end }}
{{- else if eq .Values.storageClass.mounter "s3fs" }}
  {{- if hasSuffix ".aliyuncs.com" .Values.secret.endpoint }}
    {{- .Values.storageClass.mountOptions }}
  {{- else }}
    {{- printf "-o use_path_request_style %s" .Values.storageClass.mountOptions }}
  {{- end }}
{{- else }}
  {{- .Values.storageClass.mountOptions }}
{{- end }}
{{- end }}