sts: {
	metadata: {
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
template: string
backup_policy: {
	apiVersion: "dataprotection.infracreate.com/v1alpha1"
	kind:       "BackupPolicy"
	metadata: {
		name: "\(sts.metadata.labels."app.kubernetes.io/instance")-scaling"
		namespace: sts.metadata.namespace
	}
	spec: {
		"schedule":                 "0 3 * * *"
		"ttl":                      "168h0m0s"
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
        "touch /data/mysql/data/.restore; sync"
      ],
      "postCommands": [
      ]
    }
		"onFailAttempted": 3
	}
}
