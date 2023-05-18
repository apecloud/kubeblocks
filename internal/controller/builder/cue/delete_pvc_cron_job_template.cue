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

pvc: {
	Name:      string
	Namespace: string
}
sts: {
	metadata: {
		labels: [string]: string
	}
}
cronjob: {
	apiVersion: "batch/v1"
	kind:       "CronJob"
	metadata: {
		name:      "delete-pvc-\(pvc.Name)"
		namespace: pvc.Namespace
		annotations: {
			"cluster.kubeblocks.io/lifecycle": "delete-pvc"
		}
		labels: sts.metadata.labels
	}
	spec: {
		schedule: string
		jobTemplate: {
			metadata: {
				labels: sts.metadata.labels
			}
			spec: {
				template: {
					spec: {
						containers: [
							{
								"name":            "kubectl"
								"image":           "rancher/kubectl:v1.23.7"
								"imagePullPolicy": "IfNotPresent"
								"command": [
									"kubectl", "-n", "\(pvc.Namespace)", "delete", "pvc", "\(pvc.Name)",
								]
							},
						]
						restartPolicy:  "OnFailure"
						serviceAccount: string
					}
				}
			}
		}
	}
}
