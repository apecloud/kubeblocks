{{- $bootstrap := $.Files.Get "bootstrap.yaml" | fromYamlArray }}
{{- $command := "reload" }}
{{- $trimParams := dict }}
{{- range $pk, $val := $.arg0 }}
    {{- /* trim single quotes for value in the pg config file */}}
    {{- set $trimParams $pk ( $val | trimAll "'" ) }}
    {{- if has $pk $bootstrap  }}
        {{- $command = "restart" }}
    {{- end }}
{{- end }}
{{ $params := dict "parameters" $trimParams }}
{{- $err := execSql ( dict "postgresql" $params | toJson ) "config" }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
{{- $err := execSql "" $command }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
