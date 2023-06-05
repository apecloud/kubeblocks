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

cluster: {
	metadata: {
		namespace: string
		name:      string
	}
}
component: {
	clusterDefName: string
	name:           string
	monitor: {
		enable:     bool
		builtIn:    bool
		scrapePort: int
		scrapePath: string
	}
	podSpec: containers: [...]
}

service: {
	"apiVersion": "v1"
	"kind":       "Service"
	"metadata": {
		namespace: cluster.metadata.namespace
		name:      "\(cluster.metadata.name)-\(component.name)-headless"
		labels: {
			"app.kubernetes.io/name":       "\(component.clusterDefName)"
			"app.kubernetes.io/instance":   cluster.metadata.name
			"app.kubernetes.io/managed-by": "kubeblocks"
			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
		annotations: {
			if component.monitor.enable == false {
				"prometheus.io/scrape": "false"
				"apps.kubeblocks.io/monitor": "false"
			}
			if component.monitor.enable == true && component.monitor.builtIn == false {
				"prometheus.io/scrape": "true"
				"prometheus.io/path":   component.monitor.scrapePath
				"prometheus.io/port":   "\(component.monitor.scrapePort)"
				"prometheus.io/scheme": "http"
				"apps.kubeblocks.io/monitor": "false"
			}
			if component.monitor.enable == true && component.monitor.builtIn == true {
				"prometheus.io/scrape": "false"
				"apps.kubeblocks.io/monitor": "true"
			}
		}
	}
	"spec": {
		"type":      "ClusterIP"
		"clusterIP": "None"
		"selector": {
			"app.kubernetes.io/instance":   "\(cluster.metadata.name)"
			"app.kubernetes.io/managed-by": "kubeblocks"

			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
		ports: [
			for _, container in component.podSpec.containers if container.ports != _|_
			for _, v in container.ports {
				name:       v.name
				protocol:   v.protocol
				port:       v.containerPort
				targetPort: v.name
			},
		]
	}
}
