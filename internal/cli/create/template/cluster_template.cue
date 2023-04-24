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
	name:              string
	namespace:         string
	clusterDefRef:     string
	clusterVersionRef: string
	componentSpecs: [...]
	terminationPolicy: string
	podAntiAffinity:   string
	tenancy:           string
	topologyKeys: [...]
	nodeLabels: {}
	tolerations: [...]
	annotations: {}
}

// required, k8s api resource content
content: {
	apiVersion: "apps.kubeblocks.io/v1alpha1"
	kind:       "Cluster"
	metadata: {
		name:        options.name
		namespace:   options.namespace
		annotations: options.annotations
	}
	spec: {
		clusterDefinitionRef: options.clusterDefRef
		clusterVersionRef:    options.clusterVersionRef
		affinity: {
			podAntiAffinity: options.podAntiAffinity
			topologyKeys:    options.topologyKeys
			nodeLabels:      options.nodeLabels
			tenancy:         options.tenancy
		}
		tolerations:       options.tolerations
		componentSpecs:    options.componentSpecs
		terminationPolicy: options.terminationPolicy
	}
}
