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

options: {
	name: string
	user: string

	privateKey: string
	hosts: [...Host]
	roleGroups: {}

	version: string
	criType: string
	...
}

Host: {
	name:            string
	address:         string
	internalAddress: string
}

// required, k8s api resource content
content: {
	apiVersion: "kubekey.kubesphere.io/v1alpha2"
	kind:       "Cluster"
	metadata: {
		name: options.name
	}
	spec: {
		hosts: [ for _, h in options.hosts {
			name:            h.name
			address:         h.address
			internalAddress: h.internalAddress
			if options.user != "" {
				user: options.user
			}
			if options.privateKey != "" {
				privateKey: options.privateKey
			}
		}]
		roleGroups: options.roleGroups
		controlPlaneEndpoint:
			domain: "lb.kubesphere.local"
		port: 6443
		kubernetes:
			version: options.version
		clusterName:    options.name
		autoRenewCerts: true
		if options.criType != "" {
			containerManager: options.criType
		}
		if options.criType == "" {
			containerManager: "containerd"
		}
	}
}
