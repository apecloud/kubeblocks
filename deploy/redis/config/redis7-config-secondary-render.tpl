{{- $podName := env "KB_POD_NAME" }}
{{- /* get port from env */}}
{{- $redisPort := 6379 }}
{{- $primaryPodName := env "KB_PRIMARY_POD_NAME" }}
{{- $isPrimary := false }}

{{- if or ( eq $podName "" ) ( eq $primaryPodName "" ) }}
    {{- failed "env KB_POD_NAME or KB_PRIMARY_POD_NAME is not set" }}
{{- end }}

{{- if hasPrefix $podName $primaryPodName }}
    {{- $isPrimary = true }}
{{- end }}

include "{{ $.ConfigMountPath }}/redis.conf"

{{ if $isPrimary -}}
appendonly yes
{{ else }}
replicaof {{ printf "%s %d" $primaryPodName $redisPort }}
appendonly no
{{ end }}