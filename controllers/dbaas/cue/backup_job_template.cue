sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
backup_policy_name: string
backup_job_key: {
	Name:      string
	Namespace: string
}
backup_job: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "BackupJob"
	metadata: {
		generateName: "\(backup_job_key.Name)-"
		namespace:    backup_job_key.Namespace
		labels: {
			"dataprotection.kubeblocks.io/backup-type":         "snapshot"
			"app.kubernetes.io/created-by":                     "kubeblocks"
			"backuppolicies.dataprotection.kubeblocks.io/name": backup_policy_name
			"app.kubernetes.io/created-by":                     "kubeblocks"
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		"backupPolicyName": backup_policy_name
		"backupType":       "snapshot"
	}
}
