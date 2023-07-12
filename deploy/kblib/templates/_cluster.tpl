{{/*
Define common fileds of cluster object
*/}}
{{- define "kblib.clusterCommon" }}
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: {{ include "kblib.clusterName" . }}
  namespace: {{ .Release.Namespace }}
  labels: {{ include "kblib.clusterLabels" . | nindent 4 }}
spec:
  clusterVersionRef: {{ .Values.version }}
  terminationPolicy: {{ .Values.extra.terminationPolicy }}
  {{- include "kblib.affinity" . | indent 2 }}
{{- end }}