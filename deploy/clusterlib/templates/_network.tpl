{{/*
Define component services
*/}}
{{- define "cluster-libchart.componentServices" }}
services:
  {{- if .Values.hostNetworkAccessible }}
  - name: vpc
    serviceType: LoadBalancer
    annotations:
    {{- if eq (include "cluster-libchart.cloudProvider" .) "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"
    {{- else if eq (include "cluster-libchart.cloudProvider" .) "gcp" }}
      networking.gke.io/load-balancer-type: Internal
    {{- else if eq (include "cluster-libchart.cloudProvider" .) "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
    {{- else if eq (include "cluster-libchart.cloudProvider" .) "azure" }}
     service.beta.kubernetes.io/azure-load-balancer-internal: "true"
    {{- end }}
  {{- end }}
  {{- if .Values.publiclyAccessible }}
  - name: public
    serviceType: LoadBalancer
    annotations:
    {{- if eq (include "cluster-libchart.cloudProvider" .) "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "false"
    {{- else if eq (include "cluster-libchart.cloudProvider" .) "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
    {{- else if eq (include "cluster-libchart.cloudProvider" .) "azure" }}
      service.beta.kubernetes.io/azure-load-balancer-internal: "false"
    {{- end }}
  {{- end }}
{{- end }}