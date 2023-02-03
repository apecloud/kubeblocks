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
	clusterType:    string
	type:           string
	name:           string
	// FIXME not defined in apis
	maxUnavailable: int | string
	minAvailable:   int | string
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}

pdb: {
	"apiVersion": "policy/v1"
	"kind":       "PodDisruptionBudget"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":     "\(component.clusterType)-\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component-name": "\(component.name)"
			"app.kubernetes.io/managed-by":     "kubeblocks"
		}
	}
	"spec": {
		if component.minReplicas != _|_ {
			minAvailable: component.minReplicas
		}
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":           "\(component.clusterType)-\(component.clusterDefName)"
				"app.kubernetes.io/instance":       "\(cluster.metadata.name)-\(component.name)"
				"app.kubernetes.io/component-name": "\(component.name)"
				"app.kubernetes.io/managed-by":     "kubeblocks"
			}
		}
	}
}
