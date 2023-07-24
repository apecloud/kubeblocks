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
Define the cluster role name
*/}}
{{- define "kblib.clusterRoleName" -}}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
{{- end }}

{{/*
Define the cluster role binding name
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
Define the cluster role
*/}}
{{- define "kblib.clusterRole" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ include "kblib.clusterRoleName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "kblib.clusterLabels" . | nindent 4 }}
rules:
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
{{- end }}

{{/*
Define the cluster role binding
*/}}
{{- define "kblib.clusterRoleBinding" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ include "kblib.clusterRoleBindingName" . }}
  labels:
    {{- include "kblib.clusterLabels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ include "kblib.clusterRoleName" . }}
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
{{- include "kblib.clusterRole" . }}
---
{{- include "kblib.clusterRoleBinding" . }}
{{- else }}
{{- "" }}
{{- end }}
{{- end }}
