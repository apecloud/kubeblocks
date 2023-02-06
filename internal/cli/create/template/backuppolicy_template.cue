// required, command line input options for parameters and flags
options: {
	name:             string
	namespace:        string
	clusterName:      string
	ttl:              string
	connectionSecret: string
	policyTemplate:   string
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
		backupPolicyTemplateName: options.policyTemplate
		target: {
			labelsSelector: {
				matchLabels: {
					"app.kubernetes.io/instance": options.clusterName
				}
			}
			secret: {
				name: options.connectionSecret
			}
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
