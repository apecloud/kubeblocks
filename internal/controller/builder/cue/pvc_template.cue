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
