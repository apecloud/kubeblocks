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
		labels: {
			"app.kubernetes.io/instance": string
		}
		namespace: string
	}
}
backup_key: {
	Name:      string
	Namespace: string
}
template: string
backup_policy: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "BackupPolicy"
	metadata: {
		name: backup_key.Name
		//          generateName: "\(backup_key.Name)-"
		namespace: backup_key.Namespace
		labels: {
			"app.kubernetes.io/managed-by": "kubeblocks"
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		"backupPolicyTemplateName": template
		"target": {
			"labelsSelector": {
				"matchLabels": {
					"app.kubernetes.io/instance":        sts.metadata.labels["app.kubernetes.io/instance"]
					"apps.kubeblocks.io/component-name": sts.metadata.labels["apps.kubeblocks.io/component-name"]
				}
			}
			"secret": {
				"name": "wesql-cluster"
			}
		}
		"remoteVolume": {
			"name": "backup-remote-volume"
			"persistentVolumeClaim": {
				"claimName": "backup-s3-pvc"
			}
		}
		"hooks": {
			"preCommands": [
				"touch /data/mysql/data/.restore; sync",
			]
			"postCommands": [
			]
		}
		"onFailAttempted": 3
	}
}
