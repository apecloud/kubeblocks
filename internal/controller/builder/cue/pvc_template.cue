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
		name:      string
	}
}
component: {
	clusterDefName: string
	name:           string
}
volumeClaimTemplate: {
	metadata: {
		name: string
	}
	spec: {
		accessModes: [string]
		resources: {}
	}
}
snapshot_name: string
pvc_key: {
	Name:      string
	Namespace: string
}
pvc: {
	kind:       "PersistentVolumeClaim"
	apiVersion: "v1"
	metadata: {
		name:      pvc_key.Name
		namespace: pvc_key.Namespace
		labels: {
			"apps.kubeblocks.io/vct-name": volumeClaimTemplate.metadata.name
			if component.clusterDefName != _|_ {
				"app.kubernetes.io/name":            "\(component.clusterDefName)"
			}
			if component.name != _|_ {
			  "apps.kubeblocks.io/component-name": "\(component.name)"
			}
			"app.kubernetes.io/instance":        cluster.metadata.name
			"app.kubernetes.io/managed-by":      "kubeblocks"
		}
	}
	spec: {
		accessModes: volumeClaimTemplate.spec.accessModes
		resources:   volumeClaimTemplate.spec.resources
		if len(snapshot_name) > 0 {
			dataSource: {
				"name":     snapshot_name
				"kind":     "VolumeSnapshot"
				"apiGroup": "snapshot.storage.k8s.io"
			}
		}
	}
}
