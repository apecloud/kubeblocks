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
	name:             string
	namespace:        string
	clusterName:      string
	ttl:              string
	connectionSecret: string
	policyTemplate:   string
	role:             string
}

// required, k8s api resource content
content: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "BackupPolicy"
	metadata: {
		name:      options.name
		namespace: options.namespace
	}
	spec: {
		backupPolicyTemplateName: options.policyTemplate
		target: {
			labelsSelector: {
				matchLabels: {
					"app.kubernetes.io/instance": options.clusterName
					"kubeblocks.io/role":         options.role
				}
			}
			secret: {
				name: options.connectionSecret
			}
		}
		remoteVolume: {
			name: "backup-remote-volume"
			persistentVolumeClaim: {
				claimName: "backup-s3-pvc"
			}
		}
		ttl: options.ttl
	}
}
