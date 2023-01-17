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
