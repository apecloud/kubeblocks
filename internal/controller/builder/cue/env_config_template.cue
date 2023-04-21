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
