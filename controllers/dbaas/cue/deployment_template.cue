cluster: {
	metadata: {
		namespace: string
		name:      string
	}
	spec: {
		affinity: {
			topologyKeys: [...]
			nodeLabels: {...}
		}
	}
}
component: {
	clusterDefName: string
	clusterType:    string
	type:           string
	name:           string
	podAntiAffinity: string
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}
roleGroup: {
	name:     string
	replicas: int
}

podAffinityType: *"preferredDuringSchedulingIgnoredDuringExecution" | string
if component.podAntiAffinity == "required" {
  podAffinityType: "requiredDuringSchedulingIgnoredDuringExecution"
}

deployment: {
	"apiVersion": "apps/v1"
	"kind":       "Deployment"
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
		replicas: roleGroup.replicas
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.name)-\(roleGroup.name)"
				"app.kubernetes.io/component": "\(component.type)-\(component.name)"
			}
		}
		topologySpreadConstraints: [for _, key in cluster.spec.affinity.topologyKeys {
			topologyKey: key
			maxSkew: 1
			whenUnsatisfiable: DoNotSchedule
			labelSelector: {
				matchLabels: {
				  "app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
				  "app.kubernetes.io/component": "\(component.type)-\(component.name)"
				}
			}
		}]
		affinity: {
			if len(cluster.spec.affinity.nodeLabels) > 0 {
			  nodeAffinity: {
			  	requiredDuringSchedulingIgnoredDuringExecution: {
			  		nodeSelectorTerms: [for labelKey, labelValue in cluster.spec.affinity.nodeLabels {
			  			matchExpressions: [{
			  				key: labelKey
			  				operator: "In"
			  				values: labelValue
			  			}]
			  		}]
			  	}
			  }
			}
			podAntiAffinity: {
				"\(podAffinityType)": [for _, key in cluster.spec.affinity.topologyKeys {
					podAffinityTerm: {
						labelSelector: {
							matchLabels: {
				        "app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
				        "app.kubernetes.io/component": "\(component.type)-\(component.name)"
							}
						}
						topologyKey: key
					}
					weight: 100
				}]
			}
		}
		template: {
			metadata:
				labels: {
					"app.kubernetes.io/name":      "\(component.clusterType)-\(component.clusterDefName)"
					"app.kubernetes.io/instance":  "\(cluster.metadata.name)-\(component.name)-\(roleGroup.name)"
					"app.kubernetes.io/component": "\(component.type)-\(component.name)"
					// "app.kubernetes.io/version" : # TODO
				}
			spec: component.podSpec
		}
	}
}
