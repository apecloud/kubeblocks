template: {
	name: "config-manager-sidecar"
	command: [
		"bin/reloader",
	]
	args: parameter.args

	//"registry.cn-hangzhou.aliyuncs.com/google_containers/etcd:3.5.0-0"
	image:           parameter.sidecarImage
	imagePullPolicy: "IfNotPresent"
	volumeMounts:    parameter.volumes
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
