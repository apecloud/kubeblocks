super: {
	metadata: {
		namespace: string
		name:      string
	}
	data: {...}
}

backup: {
	metadata: {
		namespace: string
		name:      string
	}
}

secret: {
	apiVersion: "v1"
	data:       super.data
	kind:       "Secret"
	metadata: {
		name:      "\(backup.metadata.name)-backup-credential"
		namespace: backup.metadata.namespace
		labels: {
			"app.kubernetes.io/managed-by":              "kubeblocks"
			"backups.dataprotection.kubeblocks.io/name": backup.metadata.name
		}
	}
}
