{{/*
Define the service account name
*/}}
{{- define "cluster-libchart.serviceAccountName" -}}
{{- printf "kb-sa-%s" (include "cluster-libchart.clusterName" .) }}
{{- end }}

{{/*
Define the role name
*/}}
{{- define "cluster-libchart.roleName" -}}
{{- printf "kb-role-%s" (include "cluster-libchart.clusterName" .) }}
{{- end }}

{{/*
Define the rolebinding name
*/}}
{{- define "cluster-libchart.roleBindingName" -}}
{{- printf "kb-rolebinding-%s" (include "cluster-libchart.clusterName" .) }}
{{- end }}

{{/*
Define the service account
*/}}
{{- define "cluster-libchart.serviceAccount" -}}
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ include "cluster-libchart.serviceAccountName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "cluster-libchart.clusterLabels" . | nindent 4 }}
{{- end }}

{{/*
Define the role
*/}}
{{- define "cluster-libchart.role" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "cluster-libchart.roleName" . }}
  namespace: {{ .Release.Namespace }}
  labels:
    {{- include "cluster-libchart.clusterLabels" . | nindent 4 }}
rules:
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
{{- end }}

{{/*
Define the rolebinding
*/}}
{{- define "cluster-libchart.roleBinding" -}}
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: {{ include "cluster-libchart.roleBindingName" . }}
  labels:
    {{- include "cluster-libchart.clusterLabels" . | nindent 4 }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: {{ include "cluster-libchart.roleName" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "cluster-libchart.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}
