//Copyright (C) 2022-2023 ApeCloud Co., Ltd
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

options: {
	name:             string
	backupPolicyName: string
	namespace:        string
	mgrNamespace:     string
	cluster:          string
	schedule:         string
	backupMethod:       string
	ttl:              string
	serviceAccount:   string
	image:            string
	tolerations: {
		tolerations: [...]
		affinity: {...}
		nodeSelector: {...}
	}
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
		successfulJobsHistoryLimit: 0
		failedJobsHistoryLimit:     1
		concurrencyPolicy:          "Forbid"
		jobTemplate: spec: template: spec: {
			restartPolicy:      "Never"
			serviceAccountName: options.serviceAccount
			affinity:           options.tolerations.affinity
			tolerations:        options.tolerations.tolerations
			nodeSelector:       options.tolerations.nodeSelector
			containers: [{
				name:            "backup-schedule"
				image:           options.image
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
    dataprotection.kubeblocks.io/backup-method: \(options.backupMethod)
    dataprotection.kubeblocks.io/autobackup: "true"
  name: backup-\(options.namespace)-\(options.cluster)-$(date -u +'%Y%m%d%H%M%S')
  namespace: \(options.namespace)
spec:
  backupPolicyName: \(options.backupPolicyName)
  backupMethod: \(options.backupMethod)
EOF
""",
				]
			}]
		}
	}
}