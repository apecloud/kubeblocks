{{/*
Define the service account name
*/}}
{{- define "kblib.serviceAccountName" -}}
{{- if .Values.extra.rbacEnabled }}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}

{{/*
Define the role name
*/}}
{{- define "kblib.roleName" -}}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
{{- end }}

{{/*
Define the rolebinding name
*/}}
{{- define "kblib.roleBindingName" -}}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
{{- end }}

{{/*
Define the clusterrolebinding name
*/}}
{{- define "kblib.clusterRoleBindingName" -}}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
{{- end }}

{{/*
Define the service account
*/}}
{{- define "kblib.serviceAccount" }}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "kblib.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "kblib.clusterLabels" . | nindent 4 }}
{{- end }}

{{/*
Define the rolebinding
*/}}
{{- define "kblib.roleBinding" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "kblib.roleBindingName" . }}
  labels:
    {{- include "kblib.clusterLabels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubeblocks-cluster-pod-role
subjects:
  - kind: ServiceAccount
    name: {{ include "kblib.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}

{{/*
Define the rolebinding
*/}}
{{- define "kblib.clusterRoleBinding" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "kblib.roleBindingName" . }}
  labels:
    {{- include "kblib.clusterLabels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kubeblocks-volume-protection-pod-role
subjects:
  - kind: ServiceAccount
    name: {{ include "kblib.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}

{{/*
Define the whole rbac
*/}}
{{- define "kblib.rbac" }}
{{- if .Values.extra.rbacEnabled }}
---
{{- include "kblib.serviceAccount" . }}
---
{{- include "kblib.clusterRoleBinding" . }}
---
{{- include "kblib.roleBinding" . }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}
