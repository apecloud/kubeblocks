sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
backup_key: {
	Name: string
	Namespace: string
}
template: string
backup_policy: {
	apiVersion: "dataprotection.infracreate.com/v1alpha1"
	kind:       "BackupPolicy"
	metadata: {
		name:      backup_key.Name
		namespace: backup_key.Namespace
	}
	spec: {
		"backupToolName":           "xtrabackup-mysql"
		"backupPolicyTemplateName": template
		"target": {
			"databaseEngine": "mysql"
			"labelsSelector": {
				"matchLabels": {
					"app.kubernetes.io/instance": sts.metadata.labels."app.kubernetes.io/instance"
				}
			}
			"secret": {
				"name":        "wesql-cluster"
				"keyUser":     "username"
				"keyPassword": "password"
			}
		}
		"remoteVolume": {
			"name": "backup-remote-volume"
			"persistentVolumeClaim": {
				"claimName": "backup-s3-pvc"
			}
		}
		"hooks": {
			"preCommands": [
				"touch /data/mysql/data/.restore; sync",
			]
			"postCommands": [
			]
		}
		"onFailAttempted": 3
	}
}
