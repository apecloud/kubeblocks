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
