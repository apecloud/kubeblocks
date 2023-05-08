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
	label?:						{}

	delay:				 		string
	errno:				 		int
	ino?: 							uint64
	size?: 						uint64
	blocks?: 					uint64
	perm?: 						uint16
	nlink?: 						uint32
	uid?: 							uint32
	gid?: 							uint32
	filling?: 				string
	maxOccurrences: 	int
	maxLength: 				int

	volumePath:				string
	path:				 			string
	percent:				 	int

	methods:					[...]
	containerNames: 	[...]
}

// required, k8s api resource content
content: {
  kind: "IOChaos"
  apiVersion: "chaos-mesh.org/v1alpha1"
  metadata:{
  	generateName: "io-chaos-"
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

 		delay: options.delay
		errno: options.errno
		if options.ino != _|_ || options.size != _|_ || options.blocks != _|_ ||
			options.perm != _|_ || options.nlink != _|_ || options.uid != _|_ || options.gid != _|_ {
				attr:{
					if options.ino != _|_ {
						ino: options.ino
					}
					if options.size != _|_ {
							size: options.size
					}
					if options.blocks != _|_ {
						blocks: options.blocks
					}
					if options.perm != _|_ {
						perm: options.perm
					}
					if options.nlink != _|_ {
						nlink: options.nlink
					}
					if options.uid != _|_ {
						uid: options.uid
					}
					if options.gid != _|_ {
						gid	: options.gid
					}
				}
		}
		if options.filling != _|_ {
			mistake:{
				filling: options.filling
				maxOccurrences: options.maxOccurrences
				maxLength: options.maxLength
			}
		}

    volumePath: options.volumePath
    path: options.path
    percent: options.percent
    methods: options.methods
    containerNames: options.containerNames
  }
}
