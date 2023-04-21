//Copyright (C) 2022 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}

component: {
	clusterDefName: string
	name:           string
}

service: {
	metadata: {
		name: string
		annotations: {}
	}
	spec: {
		ports: [...]
		type: string
	}
}

svc: {
	"apiVersion": "v1"
	"kind":       "Service"
	"metadata": {
		namespace: cluster.metadata.namespace
		if service.metadata.name != _|_ {
			name: "\(cluster.metadata.name)-\(component.name)-\(service.metadata.name)"
		}
		if service.metadata.name == _|_ {
			name: "\(cluster.metadata.name)-\(component.name)"
		}
		labels: {
			"app.kubernetes.io/name":            "\(component.clusterDefName)"
			"app.kubernetes.io/instance":        cluster.metadata.name
			"app.kubernetes.io/managed-by":      "kubeblocks"
			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
		annotations: service.metadata.annotations
	}
	"spec": {
		"selector": {
			"app.kubernetes.io/instance":        "\(cluster.metadata.name)"
			"app.kubernetes.io/managed-by":      "kubeblocks"
			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
		ports: service.spec.ports
		if service.spec.type != _|_ {
			type: service.spec.type
		}
		if service.spec.type == "LoadBalancer" {
			// Set externalTrafficPolicy to Local has two benefits:
			// 1. preserve client IP
			// 2. improve network performance by reducing one hop
			externalTrafficPolicy: "Local"
		}
	}
}
