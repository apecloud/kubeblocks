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

cluster: {
	metadata: {
		name:      string
	}
}
component: {
	clusterDefName: string
	name:           string
}
backupPolicyName: string
backupJobKey: {
	Name:      string
	Namespace: string
}
backupType: string
backupJob: {
	apiVersion: "dataprotection.kubeblocks.io/v1alpha1"
	kind:       "Backup"
	metadata: {
		name:      backupJobKey.Name
		namespace: backupJobKey.Namespace
		labels: {
			"dataprotection.kubeblocks.io/backup-type":         backupType
			"apps.kubeblocks.io/managed-by":                    "cluster"
			"backuppolicies.dataprotection.kubeblocks.io/name": backupPolicyName
			"app.kubernetes.io/name":            "\(component.clusterDefName)"
			"app.kubernetes.io/instance":        cluster.metadata.name
			"app.kubernetes.io/managed-by":      "kubeblocks"
			"apps.kubeblocks.io/component-name": "\(component.name)"
		}
	}
	spec: {
		"backupPolicyName": backupPolicyName
		"backupType":       backupType
	}
}
