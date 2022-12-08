pvc: {
	Name:      string
	Namespace: string
}
cronjob: {
  apiVersion: "batch/v1",
  kind: "CronJob",
  metadata: {
    name: "delete-pvc-\(pvc.Name)"
    namespace: pvc.Namespace
  },
  spec: {
    schedule: string,
    "jobTemplate": {
      "spec": {
        "template": {
          "spec": {
            "containers": [
              {
                "name": "kubectl",
                "image": "rancher/kubectl:v1.23.7",
                "imagePullPolicy": "IfNotPresent",
                "command": [
                		"kubectl", "-n", "\(pvc.Namespace)", "delete", "pvc", "\(pvc.Name)"
                ]
              }
            ],
            "restartPolicy": "OnFailure"
            "serviceAccount": "kubeblocks"
          }
        }
      }
    }
  }
}