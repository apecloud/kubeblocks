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
	name:           string
	clusterDefName: string
}

config: {
	apiVersion: "v1"
	kind:       "ConfigMap"
	metadata: {
		name:      "\(cluster.metadata.name)-\(component.name)-env"
		namespace: cluster.metadata.namespace
		labels: {
			"app.kubernetes.io/name":       "\(component.clusterDefName)"
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
			// configmap selector for env update
			"apps.kubeblocks.io/config-type":    "kubeblocks-env"
			"apps.kubeblocks.io/component-name": component.name
		}
	}
	data: [string]: string
}
