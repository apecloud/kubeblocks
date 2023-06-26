{{/*
Define component services
*/}}
{{- define "cluster-libchart.componentServices" }}
services:
  {{- if .Values.hostNetworkAccessible }}
  - name: vpc
    serviceType: NodePort
    annotations:
    {{- if eq "cluster-libchart.cloudProvider" "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"
    {{- else if eq "cluster-libchart.cloudProvider" "gcp" }}
      networking.gke.io/load-balancer-type: Internal
    {{- else if eq "cluster-libchart.cloudProvider" "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: intranet
    {{- else if eq "cluster-libchart.cloudPrivider" "azure" }}
     service.beta.kubernetes.io/azure-load-balancer-internal: "true"
    {{- end }}
  {{- end }}
  {{- if .Values.publiclyAccessible }}
  - name: public
    serviceType: LoadBalancer
    annotations:
    {{- if eq "cluster-libchart.cloudProvider" "aws" }}
      service.beta.kubernetes.io/aws-load-balancer-type: nlb
      service.beta.kubernetes.io/aws-load-balancer-internal: "false"
    {{- else if eq "cluster-libchart.cloudProvider" "aliyun" }}
      service.beta.kubernetes.io/alibaba-cloud-loadbalancer-address-type: internet
    {{- else if eq "cluster-libchart.cloudPrivider" "azure" }}
      service.beta.kubernetes.io/azure-load-balancer-internal: "false"
    {{- end }}
  {{- end }}
{{- end }}