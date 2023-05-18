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
	namespace: string
	action:    string
	selector: {}
	mode:     string
	value:    string
	duration: string

	delay: string
	errno: int
	attr?: {}
	mistake?: {}

	volumePath: string
	path:       string
	percent:    int
	methods: [...]
	containerNames: [...]
}

// required, k8s api resource content
content: {
	kind:       "IOChaos"
	apiVersion: "chaos-mesh.org/v1alpha1"
	metadata: {
		generateName: "io-chaos-"
		namespace:    options.namespace
	}
	spec: {
		selector: options.selector
		mode:     options.mode
		value:    options.value
		action:   options.action
		duration: options.duration

		delay: options.delay
		errno: options.errno
		if len(options.attr) != 0 {
			attr: options.attr
		}
		if len(options.mistake) != 0 {
			mistake: options.mistake
		}

		volumePath:     options.volumePath
		path:           options.path
		percent:        options.percent
		methods:        options.methods
		containerNames: options.containerNames
	}
}
