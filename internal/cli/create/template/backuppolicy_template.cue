// required, command line input options for parameters and flags
options: {
	name:             string
	namespace:        string
	clusterName:      string
	ttl:              string
	connectionSecret: string
}

// required, k8s api resource content
content: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "BackupPolicy"
	metadata: {
		name:      options.name
		namespace: options.namespace
	}
	spec: {
		schedule:       "0 3 * * *"
		backupToolName: "xtrabackup-mysql"
		target: {
			databaseEngine: "mysql"
			labelsSelector: {
				matchLabels: {
					"app.kubernetes.io/instance": options.clusterName
				}
			}
			secret: {
				name: options.connectionSecret
			}
		}

		hooks: {
			preCommands: [
				"touch /data/mysql/data/.restore_new_cluster; sync",
			]
			postCommands: [
				"rm -f /data/mysql/data/.restore_new_cluster; sync",
			]
		}

		remoteVolume: {
			name: "backup-remote-volume"
			persistentVolumeClaim: {
				claimName: "backup-s3-pvc"
			}
		}
		ttl: options.ttl
	}
}
