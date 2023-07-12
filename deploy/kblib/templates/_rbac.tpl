{{/*
Define the service account name
*/}}
{{- define "kblib.serviceAccountName" -}}
{{- printf "kb-%s" (include "kblib.clusterName" .) }}
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
Define the role
*/}}
{{- define "kblib.role" }}
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: {{ include "kblib.roleName" . }}
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
  kind: Role
  name: {{ include "kblib.roleName" . }}
subjects:
  - kind: ServiceAccount
    name: {{ include "kblib.serviceAccountName" . }}
    namespace: {{ .Release.Namespace }}
{{- end }}

{{/*
Define the whole rbac
*/}}
{{- define "kblib.rbac" }}
---
{{- include "kblib.serviceAccount" . }}
---
{{- include "kblib.role" . }}
---
{{- include "kblib.roleBinding" . }}
{{- end }}
