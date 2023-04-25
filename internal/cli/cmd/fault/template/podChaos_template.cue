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
	label:						{}
	namespaceSelector: string
	mode:							string
	value:						string
	gracePeriod: 			int
	duration: 				string
	containerNames: 	[...]
}

// required, k8s api resource content
content: {
  kind: "PodChaos"
  apiVersion: "chaos-mesh.org/v1alpha1"
  metadata:{
  	generateName: "pod-chaos-"
    namespace: options.namespace
  }
  spec:{
    selector:{
    	namespace: options.namespaceSelector
    	labelSelectors:{
    			options.label
    	}
    }
    mode: options.mode
    value: options.value
    action: options.action
    gracePeriod: options.gracePeriod
    duration: options.duration
    containerNames: options.containerNames
  }
}
