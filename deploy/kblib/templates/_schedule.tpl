{{/*
Define cluster affinity
*/}}
{{- define "kblib.affinity" }}
affinity:
  podAntiAffinity: {{ .Values.extra.podAntiAffinity }}
  topologyKeys:
  {{- if eq .Values.extra.availabilityPolicy "zone" }}
    - topology.kubernetes.io/zone
  {{- else if eq .Values.extra.availabilityPolicy "node" }}
    - kubernetes.io/hostname
  {{- end }}
  tenancy: {{ .Values.extra.tenancy }}
{{- end -}}
