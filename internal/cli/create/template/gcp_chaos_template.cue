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
	action: 					string
	secretName: 			string
	project: 					string
	zone: 						string
	instance: 				string
	deviceNames?: 			[...string]
	duration: 				string
}

// required, k8s api resource content
content: {
  kind: "GCPChaos"
  apiVersion: "chaos-mesh.org/v1alpha1"
  metadata:{
  	generateName: "gcp-chaos-"
    namespace: options.namespace
  }
  spec:{
		action: options.action
		secretName: options.secretName
		project: options.project
		zone: options.zone
		instance: options.instance
		if options.deviceNames != _|_ {
			deviceNames: options.deviceNames
		}
		duration: options.duration
	}
}
