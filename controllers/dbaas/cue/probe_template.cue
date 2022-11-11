probeContainer: {
	image: "registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.6"
	command: ["/pause"]
	imagePullPolicy: "IfNotPresent"
	name:            "string"
	readinessProbe: {
		exec: {
			command: []
		}
	}
	startupProbe: {
		tcpSocket: {
			port: 3501
		}
	}
}

