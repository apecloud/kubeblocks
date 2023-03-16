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
backup_policy_name: string
backup_job_key: {
	Name:      string
	Namespace: string
}
backup_job: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "Backup"
	metadata: {
		generateName: "\(backup_job_key.Name)-"
		namespace:    backup_job_key.Namespace
		labels: {
			"dataprotection.kubeblocks.io/backup-type":         "snapshot"
			"apps.kubeblocks.io/managed-by":                    "cluster"
			"backuppolicies.dataprotection.kubeblocks.io/name": backup_policy_name
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		"backupPolicyName": backup_policy_name
		"backupType":       "snapshot"
	}
}
