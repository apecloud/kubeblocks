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
    selector: options.selector
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
