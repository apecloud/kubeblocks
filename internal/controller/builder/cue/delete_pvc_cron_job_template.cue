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
