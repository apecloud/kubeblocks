//Copyright (C) 2022-2023 ApeCloud Co., Ltd
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
	spec: {
		clusterVersionRef: string
	}
}
component: {
	clusterDefName: string
	type:           string
	name:           string
	workloadType:   string
	replicas:       int
	podSpec: containers: [...]
	volumeClaimTemplates: [...]
}

statefulset: {
	apiVersion: "apps/v1"
	kind:       "StatefulSet"
	metadata: {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)"
		labels: {
			"app.kubernetes.io/name":            "\(component.clusterDefName)"
			"app.kubernetes.io/instance":        cluster.metadata.name
			"app.kubernetes.io/managed-by":      "kubeblocks"
			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
	}
	spec: {
		selector:
			matchLabels: {
				"app.kubernetes.io/name":            "\(component.clusterDefName)"
				"app.kubernetes.io/instance":        "\(cluster.metadata.name)"
				"app.kubernetes.io/managed-by":      "kubeblocks"
				"apps.kubeblocks.io/component-name": "\(component.name)"
			}
		serviceName:         "\(cluster.metadata.name)-\(component.name)-headless"
		replicas:            component.replicas
		minReadySeconds:     10
		podManagementPolicy: "Parallel"
		template: {
			metadata: {
				annotations: {
					"apps.kubeblocks.io/component-replicas": "\(component.replicas)"
				}
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
		volumeClaimTemplates: component.volumeClaimTemplates
	}
}
