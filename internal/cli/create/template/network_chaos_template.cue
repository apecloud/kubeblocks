// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// required, command line input options for parameters and flags
options: {
	namespace:        string
	action:						string
	selector:					{}
	mode:							string
	value:						string
	duration: 				string

	direction: 				string
	externalTargets?:  [...]

	targetMode?:				string
	targetValue:				string
	targetNamespaceSelectors: [...string]
	targetLabelSelectors:			{}

	loss?:						string
	corrupt?:					string
	duplicate?:				string

	latency?: 				string
	jitter: 					string

	correlation:			string

	rate?:						string
	limit:						uint32
	buffer:						uint32
	peakrate:					uint32
	minburst:					uint32
}

// required, k8s api resource content
content: {
  kind: "NetworkChaos"
  apiVersion: "chaos-mesh.org/v1alpha1"
  metadata:{
  	generateName: "network-chaos-"
    namespace: options.namespace
  }
  spec:{
    selector: options.selector
    mode: options.mode
    value: options.value
    action: options.action
    duration: options.duration

    direction: options.direction
    if options.externalTargets != _|_ {
  		externalTargets: options.externalTargets
    }
  	if options.targetMode != _|_ {
				target:{
					mode: options.targetMode
					value: options.targetValue
					selector:{
						namespaces: options.targetNamespaceSelectors
						labelSelectors:{
								options.targetLabelSelectors
						}
					}
				}
  	}
  	if options.loss != _|_ {
  		loss:{
  			loss: options.loss
				correlation: options.correlation
			}
  	}
		if options.corrupt != _|_ {
				corrupt:{
					corrupt: options.corrupt
					correlation: options.correlation
				}
		}
		if options.duplicate != _|_ {
				duplicate:{
					duplicate: options.duplicate
					correlation: options.correlation
				}
		}
  	if options.latency != _|_{
				delay:{
					latency: options.latency
					jitter: options.jitter
					correlation: options.correlation
				}
  	}
  	if options.rate != _|_{
				bandwidth:{
					rate: options.rate
					limit: options.limit
					buffer: options.buffer
					peakrate: options.peakrate
					minburst: options.minburst
					correlation: options.correlation
				}
		}
  }
}
