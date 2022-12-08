probeContainer: {
	image: "registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.6"
	command: ["/pause"]
	imagePullPolicy: "IfNotPresent"
	name:            "string"
	"env": [
		{
			"name": "SERVICE_USER"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "username"
					"name": "$(CONN_CREDENTIAL_SECRET_NAME)"
				}
			}
		},
		{
			"name": "SERVICE_PASSWORD"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "password"
					"name": "$(CONN_CREDENTIAL_SECRET_NAME)"
				}
			}
		},
	]
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
