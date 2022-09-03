cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}
component: {
	clusterDefName: string
	clusterType:    string
	type:           string
	name:           string
	containers: [
		{
			name:  string
			image: string
		},
	]
	volumeClaimTemplates: [...]
}
role_group: {
	name:     string
	replicas: int
}

statefulset: {
	apiVersion: "apps/v1"
	kind:       "StatefulSet"
	metadata: {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.type)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":     "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component":  "\(component.type)-\(component.name)"
			"app.kubernetes.io/created-by": "controller-manager"
		}
	}
	spec: {
		selector:
			matchLabels: {
				"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.type)-\(component.name)"
				"app.kubernetes.io/component": "\(component.type)-\(component.name)"
			}
		serviceName:     "\(cluster.metadata.name)-\(component.type)-\(component.name)"
		replicas:        role_group.replicas
		minReadySeconds: 10
		template: {
			metadata:
				labels: {
					"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
					"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.type)-\(component.name)"
					"app.kubernetes.io/component": "\(component.type)-\(component.name)"
					// "app.kubernetes.io/version" : # TODO
				}
			spec: {
				terminationGracePeriodSeconds: 10
				containers:                    component.containers
			}
		}
		volumeClaimTemplates: component.volumeClaimTemplates
	}
}
