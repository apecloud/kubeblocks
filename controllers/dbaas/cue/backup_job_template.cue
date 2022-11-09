sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
backup_job_name: string
backup_job: {
	apiVersion: "dataprotection.infracreate.com/v1alpha1"
	kind:       "BackupJob"
	metadata: {
		name:      backup_job_name
		namespace: sts.metadata.namespace
		labels: {
			"dataprotection.infracreate.com/backup-type":         "snapshot"
			"app.kubernetes.io/instance":                         sts.metadata.labels."app.kubernetes.io/instance"
			"backuppolicies.dataprotection.infracreate.com/name": "\(sts.metadata.labels."app.kubernetes.io/instance")-scaling"
			"dataprotection.infracreate.com/backup-index":        "0"
		}
	}
	spec: {
		"backupPolicyName": "\(sts.metadata.labels."app.kubernetes.io/instance")-scaling"
		"backupType":       "snapshot"
		"ttl":              "168h0m0s"
	}
}
