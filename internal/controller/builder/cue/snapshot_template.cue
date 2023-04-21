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

snapshot_key: {
	Name:      string
	Namespace: string
}
pvc_name: string
sts: {
	metadata: {
		labels: [string]: string
		namespace: string
	}
}

snapshot: {
	apiVersion: "snapshot.storage.k8s.io/v1"
	kind:       "VolumeSnapshot"
	metadata: {
		name:      snapshot_key.Name
		namespace: snapshot_key.Namespace
		labels: {
			"apps.kubeblocks.io/managed-by": "cluster"
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		source: {
			persistentVolumeClaimName: pvc_name
		}
	}
}
