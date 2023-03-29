{{- $bootstrap := $.Files.Get "bootstrap.yaml" | fromYamlArray }}
{{- $command := "reload" }}
{{- range $pk, $_ := $.arg0 }}
    {{- if has $pk $bootstrap  }}
        {{- $command = "restart" }}
        {{ break }}
    {{- end }}
{{- end }}
{{ $params := dict "parameters" $.arg0 }}
{{- $err := execSql ( dict "postgresql" $params | toJson ) "config" }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
{{- $err := execSql "" $command }}
{{- if $err }}
    {{- failed $err }}
{{- end }}
