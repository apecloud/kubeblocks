{{/*
Expand the endpoint of the secret.
*/}}
{{- define "secret.endpoint" -}}
{{- if eq .Values.secret.cloudProvider "aws" }}
  {{- if hasPrefix "cn-" .Values.secret.region }}
    {{- printf "https://s3.%s.amazonaws.com.cn" .Values.secret.region }}
  {{- else }}
    {{- printf "https://s3.%s.amazonaws.com" .Values.secret.region }}
  {{- end }}
{{- else if eq .Values.secret.cloudProvider "aliyun" }}
  {{- printf "https://oss-%s.aliyuncs.com" .Values.secret.region }}
{{- else if .Values.secret.cloudProvider }}
    fail "cloudProvider {{ .Values.secret.cloudProvider }} not supported"
{{- else }}
  {{- .Values.secret.endpoint }}
{{- end }}
{{- end }}


{{/*
Expand the mountOptions of the storageClass.
*/}}
{{- define "storageClass.mountOptions" -}}
{{- if eq .Values.storageClass.mounter "geesefs" }}
  {{- if hasSuffix ".aliyuncs.com" (include "secret.endpoint" .) }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 --subdomain %s" .Values.storageClass.mountOptions }}
  {{- else if .Values.secret.region }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 --region %s %s" .Values.secret.region .Values.storageClass.mountOptions }}
  {{- else }}
    {{- printf "--memory-limit 1000 --dir-mode 0777 --file-mode 0666 %s" .Values.storageClass.mountOptions }}
  {{- end }}
{{- else if eq .Values.storageClass.mounter "s3fs" }}
  {{- if hasSuffix ".aliyuncs.com" (include "secret.endpoint" .) }}
    {{- .Values.storageClass.mountOptions }}
  {{- else }}
    {{- printf "-o use_path_request_style %s" .Values.storageClass.mountOptions }}
  {{- end }}
{{- else }}
  {{- .Values.storageClass.mountOptions }}
{{- end }}
{{- end }}

{{/*
Create full image name
*/}}
{{- define "csi-s3.imageFullName" -}}
{{- printf "%s/%s:%s" ( .image.registry | default .root.Values.images.defaultImage.registry ) ( .image.repository ) ( .image.tag ) -}}
{{- end -}}

{{/*
Create image pull policy
*/}}
{{- define "csi-s3.imagePullPolicy" -}}
{{- printf "%s" ( .image.pullPolicy | default .root.Values.images.defaultImage.pullPolicy | default "IfNotPresent" ) -}}
{{- end -}}