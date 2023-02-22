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
	characterType:  string
	type:           string
	name:           string
	workloadType:   string
	replicas:       int
	monitor: {
		enable:     bool
		scrapePort: int
		scrapePath: string
	}
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
			"app.kubernetes.io/name":     "\(component.clusterDefName)"
			"app.kubernetes.io/instance": cluster.metadata.name
			// "app.kubernetes.io/version" : # TODO
			"app.kubernetes.io/component-name": "\(component.name)"
			"app.kubernetes.io/managed-by":     "kubeblocks"
		}
	}
	"spec": {
		replicas:        component.replicas
		minReadySeconds: 10
		selector: {
			matchLabels: {
				"app.kubernetes.io/name":           "\(component.clusterDefName)"
				"app.kubernetes.io/instance":       "\(cluster.metadata.name)"
				"app.kubernetes.io/component-name": "\(component.name)"
				"app.kubernetes.io/managed-by":     "kubeblocks"
			}
		}
		template: {
			metadata: {
				labels: {
					"app.kubernetes.io/name":           "\(component.clusterDefName)"
					"app.kubernetes.io/instance":       "\(cluster.metadata.name)"
					"app.kubernetes.io/component-name": "\(component.name)"
					"app.kubernetes.io/managed-by":     "kubeblocks"
					"kubeblocks.io/workload-type":      "\(component.workloadType)"
					// "app.kubernetes.io/version" : # TODO
				}
				if component.monitor.enable == true {
					annotations: {
						"prometheus.io/path":   component.monitor.scrapePath
						"prometheus.io/port":   "\(component.monitor.scrapePort)"
						"prometheus.io/scheme": "http"
						"prometheus.io/scrape": "true"
					}
				}
			}
			spec: component.podSpec
		}
	}
}
