{{/*
Define component services
*/}}
{{- define "cluster-libchart.componentServices" }}
services:
  {{- if .Values.hostNetworkAccessible }}
  - name: vpc
    serviceType: NodePort
  {{- end }}
  {{- if .Values.publiclyAccessible }}
  - name: public
    serviceType: LoadBalancer
{{- end }}
{{- end }}