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
	monitor: {
		enable:     bool
		scrapePort: int
		scrapePath: string
	}
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}
roleGroup: {
	replicas: int
}

deployment: {
	"apiVersion": "apps/v1"
	"kind":       "Deployment"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":     "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component-name": "\(component.name)"
			"app.kubernetes.io/created-by":     "controller-manager"
		}
	}
	"spec": {
		replicas: roleGroup.replicas
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":           "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":       "\(cluster.metadata.name)"
				"app.kubernetes.io/component-name": "\(component.name)"
			}
		}
		template: {
			metadata: {
				labels: {
					"app.kubernetes.io/name":           "\(component.clusterType)-\(component.clusterDefName)"
					"app.kubernetes.io/instance":       "\(cluster.metadata.name)"
					"app.kubernetes.io/component-name": "\(component.name)"
					// "app.kubernetes.io/version" : # TODO
				}
				if component.monitor.enable == true {
					annotations: {
						"prometheus.io/path":   component.monitor.scrapePath
						"prometheus.io/port":   "\(component.monitor.scrapePort)"
						"prometheus.io/scheme": "http"
						"prometheus.io/scrape": "true"
					}
				}
			}
			spec: component.podSpec
		}
	}
}
