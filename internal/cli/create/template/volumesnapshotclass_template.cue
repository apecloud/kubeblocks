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

// required, command line input options for parameters and flags
options: {
	driver: string
	name:   string
}

// required, k8s api resource content
content: {
	apiVersion: "snapshot.storage.k8s.io/v1"
	kind:       "VolumeSnapshotClass"
	metadata: {
		name: options.name
		annotations: {
			"snapshot.storage.kubernetes.io/is-default-class": "true"
		}
		labels: {
			"app.kubernetes.io/instance": "kubeblocks"
		}
		finalizers: ["kubeblocks.io/finalizer"]
	}
	driver:         options.driver
	deletionPolicy: "Delete"
}
