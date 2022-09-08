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
	containers: [...]
	volumeClaimTemplates: [...]
}
roleGroup: {
	updateStrategy: {
		maxUnavailable: int | string
		minAvailable:   int | string
	}
}

pdb: {
	"apiVersion": "policy/v1"
	"kind":       "PodDisruptionBudget"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)-\(roleGroup.name)"
		labels: {
			"app.kubernetes.io/name":     "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component":  "\(component.type)-\(component.name)"
			"app.kubernetes.io/created-by": "controller-manager"
		}
	}
	"spec": {
		if roleGroup.updateStrategy.minAvailable != _|_ {
			minAvailable: roleGroup.updateStrategy.minAvailable
		}
		if roleGroup.updateStrategy.maxUnavailable != _|_ {
			maxUnavailable: roleGroup.updateStrategy.maxUnavailable
		}
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.name)-\(roleGroup.name)"
				"app.kubernetes.io/component": "\(component.type)-\(component.name)"
			}
		}
	}
}
