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

options: {
	name:             string
	backupPolicyName: string
	namespace:        string
	mgrNamespace:     string
	cluster:          string
	schedule:         string
	backupType:       string
	ttl:              string
	serviceAccount:   string
}

cronjob: {
	apiVersion: "batch/v1"
	kind:       "CronJob"
	metadata: {
		name:      options.name
		namespace: options.mgrNamespace
		annotations:
			"kubeblocks.io/backup-namespace": options.namespace
		labels:
			"app.kubernetes.io/managed-by": "kubeblocks"
	}
	spec: {
		schedule:                   options.schedule
		successfulJobsHistoryLimit: 1
		failedJobsHistoryLimit:     1
		concurrencyPolicy:          "Forbid"
		jobTemplate: spec: template: spec: {
			restartPolicy:      "Never"
			serviceAccountName: options.serviceAccount
			containers: [{
				name:            "backup-policy"
				image:           "appscode/kubectl:1.25"
				imagePullPolicy: "IfNotPresent"
				command: [
					"sh",
					"-c",
				]
				args: [
					"""
kubectl create -f - <<EOF
apiVersion: dataprotection.kubeblocks.io/v1alpha1
kind: Backup
metadata:
  labels:
    app.kubernetes.io/instance: \(options.cluster)
    dataprotection.kubeblocks.io/backup-type: \(options.backupType)
    dataprotection.kubeblocks.io/autobackup: "true"
  name: backup-\(options.namespace)-\(options.cluster)-$(date -u +'%Y%m%d%H%M%S')
  namespace: \(options.namespace)
spec:
  backupPolicyName: \(options.backupPolicyName)
  backupType: \(options.backupType)
EOF
""",
				]
			}]
		}
	}
}
