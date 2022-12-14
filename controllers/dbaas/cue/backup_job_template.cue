sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
backup_job_key: {
	Name:      string
	Namespace: string
}
backup_job: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "BackupJob"
	metadata: {
		name:      backup_job_key.Name
		namespace: backup_job_key.Namespace
		labels: {
			"dataprotection.kubeblocks.io/backup-type":         "snapshot"
			"app.kubernetes.io/instance":                         sts.metadata.labels."app.kubernetes.io/instance"
			"backuppolicies.dataprotection.kubeblocks.io/name": backup_job_key.Name
			"dataprotection.kubeblocks.io/backup-index":        "0"
		}
	}
	spec: {
		"backupPolicyName": backup_job_key.Name
		"backupType":       "snapshot"
	}
}
