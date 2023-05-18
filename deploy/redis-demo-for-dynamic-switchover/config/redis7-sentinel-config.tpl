{{- $component_name := "redis" }}

{{- /* $replica := $.component.replicas */}}
{{- $replica := -1 }}
{{- $count := 0 }}
{{- range $i, $e := $.cluster.spec.componentSpecs }}
  {{- if eq $e.componentDefRef $component_name }}
    {{- $replica = $e.replicas | int }}
    {{- $count = add $count 1 }}
  {{- end }}
{{- end -}}

{{- if ne $count 1  }}
  {{- failed ( printf "not found valid clusterdef component: %s, count: %d" $component_name $count ) }}
{{- end -}}

{{- if le $replica 0  }}
  {{- failed ( printf "invalid component(%s) replicas: %d" $component_name $replica ) }}
{{- end -}}


REDIS_REPLICAS={{ $replica }}