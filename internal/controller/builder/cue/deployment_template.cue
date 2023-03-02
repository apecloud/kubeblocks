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
	spec: {
		clusterVersionRef: string
	}
}
component: {
	clusterDefName: string
	name:           string
	type:           string
	workloadType:   string
	replicas:       int
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}

deployment: {
	"apiVersion": "apps/v1"
	"kind":       "Deployment"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":       "\(component.clusterDefName)"
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"

			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
	}
	"spec": {
		replicas:        component.replicas
		minReadySeconds: 10
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":       "\(component.clusterDefName)"
				"app.kubernetes.io/instance":   "\(cluster.metadata.name)"
				"app.kubernetes.io/managed-by": "kubeblocks"

				"apps.kubeblocks.io/component-name": "\(component.name)"
			}
		}
		template: {
			metadata: {
				labels: {
					"app.kubernetes.io/name":       "\(component.clusterDefName)"
					"app.kubernetes.io/instance":   "\(cluster.metadata.name)"
					"app.kubernetes.io/managed-by": "kubeblocks"
					"app.kubernetes.io/component":  "\(component.type)"
					if cluster.spec.clusterVersionRef != _|_ {
						"app.kubernetes.io/version": "\(cluster.spec.clusterVersionRef)"
					}
					"apps.kubeblocks.io/component-name": "\(component.name)"
					"apps.kubeblocks.io/workload-type":  "\(component.workloadType)"
				}
			}
			spec: component.podSpec
		}
	}
}
