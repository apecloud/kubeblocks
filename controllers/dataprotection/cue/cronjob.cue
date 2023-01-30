options: {
	name:       string
	namespace:  string
	cluster:    string
	schedule:   string
	backupType: string
	ttl:        string
}

cronjob: {
	apiVersion: "batch/v1"
	kind:       "CronJob"
	metadata: {
		name:      options.name
		namespace: options.namespace
		labels:
			"app.kubernetes.io/managed-by": "kubeblocks"
	}
	spec: {
		schedule:                   options.schedule
		successfulJobsHistoryLimit: 1
		failedJobsHistoryLimit:     1
		concurrencyPolicy:          "Forbid"
		jobTemplate: spec: template: spec: {
			restartPolicy:      "Never"
			serviceAccount:     "kubeblocks"
			serviceAccountName: "kubeblocks"
			containers: [{
				name:            "backup-policy"
				image:           "appscode/kubectl:1.25"
				imagePullPolicy: "IfNotPresent"
				command: [
					"sh",
					"-c",
				]
				args: [
					"""
kubectl create -f - <<EOF
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  labels:
    app.kubernetes.io/instance: \(options.cluster)
    dataprotection.kubeblocks.io/backup-type: \(options.backupType)
    dataprotection.kubeblocks.io/autobackup: "true"
  name: backup-\(options.namespace)-\(options.cluster)-$(date -u +'%Y%m%d%H%M%S')
  namespace: default
spec:
  backupPolicyName: \(options.name)
  backupType: \(options.backupType)
  ttl: \(options.ttl)
EOF
""",
				]
			}]
		}
	}
}
