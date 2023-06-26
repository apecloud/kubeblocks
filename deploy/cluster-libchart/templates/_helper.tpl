{{/*
Get cloud provider, now support aws, gcp, aliyun and tencentCloud. For azure, we should get it from node.Spec.ProviderID
*/}}
{{- define "cluster-libchart.cloudProvider" }}
{{- $kubeVersion := .Capabilities.KubeVersion.GitVersion }}
{{- if contains $kubeVersion "eks" }}
{{- "aws" -}}
{{- else if contains $kubeVersion "gke" }}
{{- "gcp" -}}
{{- else if contains $kubeVersion "aliyun" }}
{{- "aliyun" -}}
{{- else if contains $kubeVersion "tke" }}
{{- "tencentCloud" -}}
{{- else }}
{{- "" -}}
{{- end }}
{{- end }}