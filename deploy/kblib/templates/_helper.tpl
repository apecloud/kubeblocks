{{/*
Get cloud provider, now support aws, gcp, aliyun and tencentCloud.
TODO: For azure, we should get provider from node.Spec.ProviderID
*/}}
{{- define "kblib.cloudProvider" }}
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