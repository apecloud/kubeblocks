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
	kind:        string
	namespace:   string
	action:      string
	secretName:  string
	region:      string
	instance:    string
	volumeID:    string
	deviceName?: string
	duration:    string

	project: string
}

// required, k8s api resource content
content: {
	kind:       options.kind
	apiVersion: "chaos-mesh.org/v1alpha1"
	metadata: {
		generateName: "node-chaos-"
		namespace:    options.namespace
	}
	spec: {
		action:     options.action
		secretName: options.secretName
		duration:   options.duration

		if options.kind == "AWSChaos" {
			awsRegion:   options.region
			ec2Instance: options.instance
			if options.deviceName != _|_ {
				volumeID:   options.volumeID
				deviceName: options.deviceName
			}
		}

		if options.kind == "GCPChaos" {
			project:  options.project
			zone:     options.region
			instance: options.instance
			if options.deviceName != _|_ {
				deviceNames: [options.deviceName]
			}

		}
	}
}
