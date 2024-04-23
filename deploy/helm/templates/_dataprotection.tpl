{{/*
Create the name of the ServiceAccount for worker pods.
*/}}
{{- define "dataprotection.workerSAName" -}}
{{- if .Values.dataProtection.worker.serviceAccount.name }}
{{- .Values.dataProtection.worker.serviceAccount.name }}
{{- else }}
{{- include "kubeblocks.fullname" . }}-dataprotection-worker
{{- end }}
{{- end }}

{{/*
Create the name of the ServiceAccount for worker pods that runs "kubectl exec".
*/}}
{{- define "dataprotection.execWorkerSAName" -}}
{{- include "kubeblocks.fullname" . }}-dataprotection-exec-worker
{{- end }}

{{/*
Create the name of the ClusterRole for worker pods.
*/}}
{{- define "dataprotection.workerClusterRoleName" -}}
{{- include "kubeblocks.fullname" . }}-dataprotection-worker-role
{{- end }}

{{/*
Create the name of the Role for exec worker pods.
*/}}
{{- define "dataprotection.execWorkerRoleName" -}}
{{- include "kubeblocks.fullname" . }}-dataprotection-exec-worker-role
{{- end }}

{{/*
Render the secret key reference of the encryptionKey.
*/}}
{{- define "dataprotection.encryptionKeySecretKeyRef" -}}
  {{- $ref := .Values.dataProtection.encryptionKeySecretKeyRef -}}
  {{- if or (eq $ref.name "") (eq $ref.key "") -}}
name: {{ include "kubeblocks.fullname" . }}-secret
key: dataProtectionEncryptionKey
  {{- else -}}
    {{- $secret := lookup "v1" "Secret" .Release.Namespace $ref.name -}}
    {{- if not $secret -}}
      {{- fail (printf "Invalid value \".Values.dataProtection.encryptionKeySecretKeyRef\", secret %q is not found from the namespace %q." $ref.name .Release.Namespace) -}}
    {{- else if not (hasKey $secret.data $ref.key) -}}
      {{- fail (printf "Invalid value \".Values.dataProtection.encryptionKeySecretKeyRef\", secret %q doesn't have key %q." $ref.name $ref.key) -}}
    {{- end -}}
name: {{ $ref.name }}
key:  {{ $ref.key }}
  {{- end -}}
{{- end }}

{{/*
Render the algorithm for backup encryption, empty if not specified or invalid.
*/}}
{{- define "dataprotection.backupEncryptionAlgorithm" -}}
  {{- $allowed := list "AES-128-CFB" "AES-192-CFB" "AES-256-CFB" -}}
  {{- if has .Values.dataProtection.backupEncryptionAlgorithm $allowed -}}
    {{ .Values.dataProtection.backupEncryptionAlgorithm | quote }}
  {{- else -}}
    {{ "" | quote }}
  {{- end -}}
{{- end }}
