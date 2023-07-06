{{/*
Define component services
*/}}
{{- define "kblib.componentServices" }}
services:
  {{- if .Values.extra.hostNetworkAccessible }}
  - name: vpc
    serviceType: LoadBalancer
    annotations:
    {{- if eq (include "kblib.cloudProvider" .) "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"
    {{- else if eq (include "kblib.cloudProvider" .) "gcp" }}
      networking.gke.io/load-balancer-type: Internal
    {{- else if eq (include "kblib.cloudProvider" .) "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
    {{- else if eq (include "kblib.cloudProvider" .) "azure" }}
     service.beta.kubernetes.io/azure-load-balancer-internal: "true"
    {{- end }}
  {{- end }}
  {{- if .Values.extra.publiclyAccessible }}
  - name: public
    serviceType: LoadBalancer
    annotations:
    {{- if eq (include "kblib.cloudProvider" .) "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "false"
    {{- else if eq (include "kblib.cloudProvider" .) "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
    {{- else if eq (include "kblib.cloudProvider" .) "azure" }}
      service.beta.kubernetes.io/azure-load-balancer-internal: "false"
    {{- end }}
  {{- end }}
{{- end }}