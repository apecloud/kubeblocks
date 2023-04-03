container: {
	name:            "pitr"
	image:           string
	imagePullPolicy: "IfNotPresent"
	command: [...]
	args: [...]
	volumeMounts: [...]
}

job: {
	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: {
		name:      string
		namespace: string
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
