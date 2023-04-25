{{- $podName := env "KB_POD_NAME" }}
{{- $primaryPodName := env "KB_PRIMARY_POD_NAME" }}
{{- $isPrimary := false }}

{{- if hasPrefix $podName $primaryPodName }}
    {{- $isPrimary = true }}
{{- end }}

include {{ .configMountPath }}/redis.conf"

{{- if $isPrimary }}
appendonly yes
{{- else }}
replicaof {{ $$primaryPodName }} 6379
appendonly no
{{- end }}
