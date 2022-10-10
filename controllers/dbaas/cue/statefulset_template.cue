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
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}
roleGroup: {
	name:     string
	replicas: int
}

statefulset: {
	apiVersion: "apps/v1"
	kind:       "StatefulSet"
	metadata: {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
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
				"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.name)"
				"app.kubernetes.io/component": "\(component.type)-\(component.name)"
			}
		serviceName:         "\(cluster.metadata.name)-\(component.name)"
		replicas:            roleGroup.replicas
		minReadySeconds:     10
		podManagementPolicy: "Parallel"
		template: {
			metadata:
				labels: {
					"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
					"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.name)"
					"app.kubernetes.io/component": "\(component.type)-\(component.name)"
					// "app.kubernetes.io/version" : # TODO
				}
			spec: component.podSpec
		}
		volumeClaimTemplates: component.volumeClaimTemplates
	}
}
