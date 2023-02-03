// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// required, command line input options for parameters and flags
options: {
	name:                   string
	namespace:              string
	opsRequestName:         string
	type:                   string
	typeLower:              string
	ttlSecondsAfterSucceed: int
	clusterVersionRef:      string
	componentNames: [...string]
	requestCPU:    string
	requestMemory: string
	limitCPU:      string
	limitMemory:   string
	replicas:      int
	storage:       string
	vctNames: [...string]
	keyValues: [string]: string
	cfgTemplateName: string
	cfgFile:         string
	...
}

// define operation block,
#upgrade: {
	clusterVersionRef: options.clusterVersionRef
}

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
			upgrade: #upgrade
		}
		if options.type == "VolumeExpansion" {
			volumeExpansion: [ for _, cName in options.componentNames {
				componentName: cName
				volumeClaimTemplates: [ for _, vctName in options.vctNames {
					name:    vctName
					storage: options.storage
				}]
			}]
		}
		if options.type == "HorizontalScaling" {
			horizontalScaling: [ for _, cName in options.componentNames {
				componentName: cName
				replicas:      options.replicas
			}]
		}
		if options.type == "Restart" {
			restart: [ for _, cName in options.componentNames {
				componentName: cName
			}]
		}
		if options.type == "VerticalScaling" {
			verticalScaling: [ for _, cName in options.componentNames {
				componentName: cName
				requests: {
					if options.requestMemory != "" {
						memory: options.requestMemory
					}
					if options.requestCPU != "" {
						cpu: options.requestCPU
					}
				}
				limits: {
					if options.limitMemory != "" {
						memory: options.limitMemory
					}
					if options.limitCPU != "" {
						cpu: options.limitCPU
					}
				}
			}]
		}
		if options.type == "Reconfiguring" {
			reconfigure: {
				componentName: options.componentNames[0]
				configurations: [ {
					name: options.cfgTemplateName
					keys: [{
						key: options.cfgFile
						parameters: [ for k, v in options.keyValues {
							key:   k
							value: v
						}]
					}]
				}]
			}
		}
	}
}