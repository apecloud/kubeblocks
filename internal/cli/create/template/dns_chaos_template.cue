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

	patterns: [...]
}

// required, k8s api resource content
content: {
	kind:       "DNSChaos"
	apiVersion: "chaos-mesh.org/v1alpha1"
	metadata: {
		generateName: "dns-chaos-"
		namespace:    options.namespace
	}
	spec: {
		selector: options.selector
		action:   options.action
		mode:     options.mode
		value:    options.value
		duration: options.duration

		patterns: options.patterns
	}
}
