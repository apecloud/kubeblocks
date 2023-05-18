{{- /* get port from env */}}
{{- $redisPort := 6379 -}}

include "{{ $.ConfigMountPath }}/redis.conf"

{{- if eq $.role "primary" }}
appendonly yes
{{ else }}
replicaof {{ printf "%s %d" $.primary $redisPort }}
appendonly no
{{ end }}