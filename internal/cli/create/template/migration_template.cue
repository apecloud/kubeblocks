//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
