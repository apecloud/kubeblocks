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
	class:    string
	classDefRef: {...}
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
					classDefRef: options.classDefRef
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
