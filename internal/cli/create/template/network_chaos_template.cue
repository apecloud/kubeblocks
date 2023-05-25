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
	target?: 						{}

	loss?: 							{}
	delay?: 						{}
	duplicate?:					{}
	corrupt?:						{}
	bandwidth?:					{}
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

    if options.target["mode"] != _|_ || len(options.target["selector"]) !=0 {
			target: options.target
		}

		if options.loss["loss"] != _|_ {
			loss: options.loss
		}

		if options.delay["latency"] != _|_ {
			delay: options.delay
		}

		if options.corrupt["corrupt"] != _|_ {
			corrupt: options.corrupt
		}

		if options.duplicate["duplicate"] != _|_ {
			duplicate: options.duplicate
		}

		if options.bandwidth["rate"] != _|_ {
			bandwidth: options.bandwidth
		}
	}
}
