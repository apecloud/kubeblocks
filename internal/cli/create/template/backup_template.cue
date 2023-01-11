// required, command line input options for parameters and flags
options: {
	backupName:   string
	namespace:    string
	backupType:   string
	backupPolicy: string
	ttl:          string
}

// required, k8s api resource content
content: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "Backup"
	metadata: {
		name:      options.backupName
		namespace: options.namespace
	}
	spec: {
		backupType:       options.backupType
		backupPolicyName: options.backupPolicy
		ttl:              options.ttl
	}
}
