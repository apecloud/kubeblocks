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
	name:      string
	namespace: string
	teplate:   string
	taskType:  string
	source:    string
	sourceEndpointModel: {}
	sink: string
	sinkEndpointModel: {}
	migrationObject: [...]
	migrationObjectModel: {}
	steps: [...]
	stepsModel: [...]
	tolerations: [...]
	tolerationModel: {}
	resources: [...]
	resourceModel: {}
	serverId: int
}

// required, k8s api resource content
content: {
	apiVersion: "datamigration.apecloud.io/v1alpha1"
	kind:       "MigrationTask"
	metadata: {
		name:      options.name
		namespace: options.namespace
	}
	spec: {
		taskType:       options.taskType
		template:       options.template
		sourceEndpoint: options.sourceEndpointModel
		sinkEndpoint:   options.sinkEndpointModel
		initialization: {
			steps: options.stepsModel
			config: {
				preCheck: {
					resource: {
						limits: options.resourceModel["precheck"]
					}
					tolerations: options.tolerationModel["precheck"]
				}
				initStruct: {
					resource: {
						limits: options.resourceModel["init-struct"]
					}
					tolerations: options.tolerationModel["init-struct"]
				}
				initData: {
					resource: {
						limits: options.resourceModel["init-data"]
					}
					tolerations: options.tolerationModel["init-data"]
				}
			}
		}
		cdc: {
			config: {
				resource: {
					limits: options.resourceModel["cdc"]
				}
				tolerations: options.tolerationModel["cdc"]
				param: {
					"extractor.server_id": options.serverId
				}
			}
		}
		migrationObj:      options.migrationObjectModel
		globalTolerations: options.tolerationModel["global"]
		globalResources: {
			limits: options.resourceModel["global"]
		}
	}
}
