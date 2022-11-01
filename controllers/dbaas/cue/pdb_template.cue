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
	// FIXME not defined in apis
	maxUnavailable: int | string
	minAvailable:   int | string
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}

pdb: {
	"apiVersion": "policy/v1"
	"kind":       "PodDisruptionBudget"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":     "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component-name": "\(component.name)"
		}
	}
	"spec": {
		if component.minAvailable != _|_ {
			minAvailable: component.minAvailable
		}
		if component.maxUnavailable != _|_ {
			maxUnavailable: component.maxUnavailable
		}
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":           "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":       "\(cluster.metadata.name)-\(component.name)"
				"app.kubernetes.io/component-name": "\(component.name)"
			}
		}
	}
}
