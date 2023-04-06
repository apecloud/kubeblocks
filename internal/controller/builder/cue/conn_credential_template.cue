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

clusterdefinition: {
	metadata: {
		name: string
	}
	spec: {
		type: string
		connectionCredential: {...}
	}
}
cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}
secret: {
	apiVersion: "v1"
	stringData: clusterdefinition.spec.connectionCredential
	kind:       "Secret"
	metadata: {
		name:      "\(cluster.metadata.name)-conn-credential"
		namespace: cluster.metadata.namespace
		labels: {
			"app.kubernetes.io/name":       "\(clusterdefinition.metadata.name)"
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
			if clusterdefinition.spec.type != _|_ {
			  "apps.kubeblocks.io/character-type": clusterdefinition.spec.type
			}
		}
	}
}
