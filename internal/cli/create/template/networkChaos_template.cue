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
	namespace:        string
	action:						string

	namespaceSelector: [...]
	mode:							string
	value:						string
	duration: 				string
	// optional
	label?:						{}

	direction: 				string
	// // optional
	externalTargets?:  [...]

	targetNamespaceSelector: string
	targetMode:				string
	targetValue:			string
	targetLabel?:			{}

	loss?:						string
	corrupt?:					string
	duplicate?:				string

	latency?: 				string
	jitter: 					string

	correlation:			string
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
    selector:{
    	namespaces: options.namespaceSelector
			if options.label != _|_ {
				labelSelectors:{
					options.label
					}
			}
    }
    mode: options.mode
    value: options.value
    action: options.action
    duration: options.duration

    direction: options.direction
    if options.externalTargets != _|_ {
  		externalTargets: options.externalTargets
    }
  	if options.targetLabel != _|_ {
				target:{
					mode: options.targetMode
					value: options.targetValue
					selector:{
						namespaces: [options.targetNamespaceSelector]
						labelSelectors:{
								options.targetLabel
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
  }
}
