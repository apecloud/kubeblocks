// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

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
backup_type: string
backup_job: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "Backup"
	metadata: {
		generateName: "\(backup_job_key.Name)-"
		namespace:    backup_job_key.Namespace
		labels: {
			"dataprotection.kubeblocks.io/backup-type":         backup_type
			"apps.kubeblocks.io/managed-by":                    "cluster"
			"backuppolicies.dataprotection.kubeblocks.io/name": backup_policy_name
			for k, v in sts.metadata.labels {
				"\(k)": "\(v)"
			}
		}
	}
	spec: {
		"backupPolicyName": backup_policy_name
		"backupType":       backup_type
	}
}
