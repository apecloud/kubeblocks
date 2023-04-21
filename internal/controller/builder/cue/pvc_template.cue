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

sts: {
	metadata: {
		labels: [string]: string
	}
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
			"vct.kubeblocks.io/name": volumeClaimTemplate.metadata.name
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		accessModes: volumeClaimTemplate.spec.accessModes
		resources:   volumeClaimTemplate.spec.resources
		dataSource: {
			"name":     snapshot_name
			"kind":     "VolumeSnapshot"
			"apiGroup": "snapshot.storage.k8s.io"
		}
	}
}
