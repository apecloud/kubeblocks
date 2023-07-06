{{/*
Define cluster affinity
*/}}
{{- define "kblib.affinity" }}
affinity:
  podAntiAffinity: Preferred
  {{- if eq .Values.extra.availabilityPolicy "zone" }}
  topologyKeys:
    - topology.kubernetes.io/zone
  {{- else if eq .Values.extra.availabilityPolicy "node" }}
    - kubernetes.io/hostname
  {{- end }}
  tenancy: {{ .Values.extra.tenancy }}
{{- end -}}
