cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}
component: {
	name: string
	type: string
}

config: {
	apiVersion: "v1"
	kind:       "ConfigMap"
	metadata: {
		name:      "\(cluster.metadata.name)-\(component.name)-env"
		namespace: cluster.metadata.namespace
		labels: {
			"app.kubernetes.io/name":           "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance":       cluster.metadata.name
			"app.kubernetes.io/component-name": component.name
			"app.kubernetes.io/component":      "\(component.type)-\(component.name)"
			"app.kubernetes.io/managed-by":     "kubeblocks"
			// configmap selector for env update
			"app.kubernetes.io/config-type": "kubeblocks-env"
		}
	}
	data: [string]: string
}
