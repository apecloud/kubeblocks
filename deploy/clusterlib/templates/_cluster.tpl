{{/*
Define common fileds of cluster object
*/}}
{{- define "cluster-libchart.clusterCommon" }}
apiVersion: apps.kubeblocks.io/v1alpha1
kind: Cluster
metadata:
  name: {{ include "cluster-libchart.clusterName" . }}
  labels: {{ include "cluster-libchart.clusterLabels" . | nindent 4 }}
spec:
  clusterVersionRef: {{ .Values.version }}
  terminationPolicy: {{ .Values.terminationPolicy }}
  {{- include "cluster-libchart.affinity" . | indent 2 }}
{{- end }}