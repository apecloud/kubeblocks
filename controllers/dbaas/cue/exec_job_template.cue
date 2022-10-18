cmd: {
	args: [string]
}
pod: {
	metadata: {
		name: string
		namespace: string
	}
	spec: {
		containers: [...]
		volumes: [...]
	}
}
job: {
	apiVersion: "batch/v1"
	kind:       "Job"
	metadata: {
		name: pod.metadata.name
		namespace: pod.metadata.namespace
	}
	spec: {
		template: {
			spec: {
				containers: [
					{
						name:  "kubectl"
						image: "rancher/kubectl:v1.23.7"
						command: [
							"kubectl",
							"exec",
							"-i",
							pod.metadata.name,
							"-c",
							pod.spec.containers[0].name,
							"--",
							"sh",
							"-c",
						]
						args: cmd.args
						volumeMounts: pod.spec.containers[0].volumeMounts
					},
				]
				volumes: pod.spec.volumes
				restartPolicy: "Never"
				serviceAccountName: "opendbaas-core"
			}
		}
		"backoffLimit": 4
	}
}
