// required, command line input options for parameters and flags
options: {
	name:                   string
	namespace:              string
	opsRequestName:         string
	type:                   string
	typeLower:              string
	ttlSecondsAfterSucceed: int
	appVersionRef:          string
	componentNames: [...string]
	requestCpu:    string
	requestMemory: string
	limitCpu:      string
	limitMemory:   string
	replicas:      int
	roleGroupNames: [...string]
	roleGroupReplicas: int
	storage:           string
	vctNames: [...string]
}

// define operation block,
#upgrade: {
	appVersionRef: options.appVersionRef
}

#componentOps: [
	{
		componentNames: options.componentNames

		// define details api block with options.type
		if options.type == "VerticalScaling" {
			verticalScaling: {
				requests: {
					if options.requestMemory != "" {
						memory: options.requestMemory
					}
					if options.requestCpu != "" {
						cpu: options.requestCpu
					}

				}
				limits: {
					if options.limitMemory != "" {
						memory: options.limitMemory
					}
					if options.limitCpu != "" {
						cpu: options.limitCpu
					}
				}
			}
		}

		if options.type == "VolumeExpansion" {
			volumeExpansion: [ for _, vctName in options.vctNames {
				name:    vctName
				storage: options.storage
			}]
		}

		if options.type == "HorizontalScaling" {
			horizontalScaling: {
				if options.replicas >= 0 {
					replicas: options.replicas
				}
				roleGroups: [ for _, roleGroupName in options.roleGroupNames {
					name:     roleGroupName
					replicas: options.roleGroupReplicas
				}]
			}
		}
	},
]

// required, k8s api resource content
content: {
	apiVersion: "dbaas.kubeblocks.io/v1alpha1"
	kind:       "OpsRequest"
	metadata: {
		if options.opsRequestName == "" {
			generateName: "opsrequest-\(options.typeLower)-"
		}
		if options.opsRequestName != "" {
			name: options.opsRequestName
		}
		namespace: options.namespace
	}
	spec: {
		clusterRef:             options.name
		type:                   options.type
		ttlSecondsAfterSucceed: options.ttlSecondsAfterSucceed
		if options.type == "Upgrade" {
			clusterOps: {
				upgrade: #upgrade
			}
		}
		if options.type != "Upgrade" {
			componentOps: #componentOps
		}
	}
}
