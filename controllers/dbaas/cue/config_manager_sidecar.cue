template: {
	name: "config-manager-sidecar"
	command: [
		"/bin/reloader",
	]
	args: parameter.args
	env: [
		{
			name: "CONFIG_MANAGER_POD_IP"
			valueFrom:
				fieldRef:
					fieldPath: "status.podIP"
		},
	]

	image:           parameter.sidecarImage
	imagePullPolicy: "IfNotPresent"
	volumeMounts:    parameter.volumes
	securityContext:
		runAsUser: 0
	defaultAllowPrivilegeEscalation: false
}

#ArgType: string
#EnvType: {
	name:  string
	value: string

	// valueFrom
	...
}

parameter: {
	name:         string
	sidecarImage: string
	args: [...#ArgType]
	// envs?: [...#EnvType]
	volumes: [...]
}
