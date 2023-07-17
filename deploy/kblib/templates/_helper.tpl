{{/*
Get cloud provider, now support aws, gcp, aliyun and tencentCloud.
TODO: For azure, we should get provider from node.Spec.ProviderID
*/}}
{{- define "kblib.cloudProvider" }}
{{- $kubeVersion := .Capabilities.KubeVersion.GitVersion }}
{{- if contains "eks" $kubeVersion }}
{{- "aws" -}}
{{- else if contains "gke" $kubeVersion }}
{{- "gcp" -}}
{{- else if contains "aliyun" $kubeVersion }}
{{- "aliyun" -}}
{{- else if contains "tke" $kubeVersion }}
{{- "tencentCloud" -}}
{{- else }}
{{- "" -}}
{{- end }}
{{- end }}