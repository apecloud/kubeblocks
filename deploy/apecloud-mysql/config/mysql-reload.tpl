{{- /* mysql global variable update */}}
{{- /* mysql using system variables reference docs: https://dev.mysql.com/doc/refman/8.0/en/using-system-variables.html */}}
{{- /*  1. system variable names must be written using underscores, not dashes. */}}
{{- /*  2. string variable 'xxx' */}}
{{- /*  3. type convert to number */}}
{{- range $pk, $pv := $.arg0 }}
	{{- $pk = trimPrefix "loose_" $pk }}
	{{- $pk = replace "-" "_" $pk }}
	{{- $var_int := -1 }}
    {{- if $pv | regexMatch "^\\d+$" }}
		{{- $var_int = atoi $pv }}
	{{- end}}
	{{- if lt $var_int 0 }}
		{{- $tmp := $pv | regexStringSubmatch "^(\\d+)K$" }}
		{{- if $tmp }}
		{{- $var_int = last $tmp | atoi | mul 1024 }}
		{{- end }}
	{{- end }}
	{{- if lt $var_int 0 }}
		{{- $tmp := $pv | regexStringSubmatch "^(\\d+)M$" }}
		{{- if $tmp }}
		{{- $var_int =  last $tmp | atoi | mul 1024 1024 }}
		{{- end }}
	{{- end }}
	{{- if lt $var_int 0 }}
		{{- $tmp := $pv | regexStringSubmatch "^(\\d+)G$" }}
		{{- if $tmp }}
		{{- $var_int = last $tmp | atoi | mul 1024 1024 1024 }}
		{{- end }}
	{{- end }}
	{{- if ge $var_int 0 }}
		{{- execSql ( printf "SET GLOBAL %s = %d" $pk $var_int ) }}
	{{- else }}
		{{- execSql ( printf "SET GLOBAL %s = '%s'" $pk $pv ) }}
	{{- end }}
{{- end }}