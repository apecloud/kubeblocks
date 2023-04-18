container: {
	name:            "pitr"
	image:           string
	imagePullPolicy: "IfNotPresent"
	command: [...]
	args: [...]
	volumeMounts: [...]
	env: [...]
}

job: {
	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: {
		name:      string
		namespace: string
		labels: {
			"app.kubernetes.io/managed-by": "kubeblocks"
		}
	}
	spec: {
		template: {
			spec: {
				containers: [container]
				volumes: [...]
				restartPolicy: "OnFailure"
				securityContext:
					runAsUser: 0
			}
		}
	}
}
