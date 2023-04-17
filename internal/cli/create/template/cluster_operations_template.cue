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
	cpu:      string
	memory:   string
	class: 	  string
	replicas: int
	storage:  string
	vctNames: [...string]
	keyValues: [string]: string
	cfgTemplateName: string
	cfgFile:         string
	services: [
		...{
			name:        string
			serviceType: string
			annotations: {...}
		},
	]
	...
}

// define operation block,
#upgrade: {
	clusterVersionRef: options.clusterVersionRef
}

// required, k8s api resource content
content: {
	apiVersion: "apps.kubeblocks.io/v1alpha1"
	kind:       "OpsRequest"
	metadata: {
		if options.opsRequestName == "" {
			generateName: "\(options.name)-\(options.typeLower)-"
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
				if options.class != "" {
					class: options.class
				}
				requests: {
					if options.memory != "" {
						memory: options.memory
					}
					if options.cpu != "" {
						cpu: options.cpu
					}
				}
				limits: {
					if options.memory != "" {
						memory: options.memory
					}
					if options.cpu != "" {
						cpu: options.cpu
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
		if options.type == "Expose" {
			expose: [ for _, cName in options.componentNames {
				componentName: cName
				services: [ for _, svc in options.services {
					name:        svc.name
					serviceType: svc.serviceType
					annotations: svc.annotations
				}]
			}]
		}
	}
}
