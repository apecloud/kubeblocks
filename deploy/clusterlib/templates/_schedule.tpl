{{/*
Define cluster affinity
*/}}
{{- define "cluster-libchart.affinity" }}
affinity:
  podAntiAffinity: Preferred
  {{- if eq .Values.availabilityPolicy "zone" }}
  topologyKeys:
    - topology.kubernetes.io/zone
  {{- else if eq .Values.availabilityPolicy "node" }}
    - kubernetes.io/hostname
  {{- end }}
  tenancy: {{ .Values.tenancy }}
{{- end -}}
