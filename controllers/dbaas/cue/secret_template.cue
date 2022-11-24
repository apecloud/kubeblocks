clusterdefinition: {
	metadata: {
		name: string
	}
	spec: {
		type: string
		connectionCredential: {
			user: string
		}
	}
}
cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}
secret: {
	apiVersion: "v1"
	stringData: {
		username: clusterdefinition.spec.connectionCredential.user | *"root"
		password: string
	}
	kind: "Secret"
	metadata: {
		name:      cluster.metadata.name
		namespace: cluster.metadata.namespace
		labels: {
			"app.kubernetes.io/name":       "\(clusterdefinition.spec.type)-\(clusterdefinition.metadata.name)"
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
			// "app.kubernetes.io/version" : # TODO
		}
	}
}
