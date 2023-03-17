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
