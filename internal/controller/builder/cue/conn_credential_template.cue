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
				"apps.kubeblocks.io/cluster-type": clusterdefinition.spec.type
			}
		}
	}
}
