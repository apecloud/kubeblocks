monitor: {
	secretName:      string
	internalPort:    int
	image:           string
	imagePullPolicy: string
}

container: {
	"name":            "inject-mysql-exporter"
	"imagePullPolicy": "\(monitor.imagePullPolicy)"
	"image":           "\(monitor.image)"
	"command": ["/bin/agamotto"]
	"env": [
		{
			"name": "MYSQL_USER"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "username"
					"name": "$(CONN_CREDENTIAL_SECRET_NAME)"
				}
			}
		},
		{
			"name": "MYSQL_PASSWORD"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "password"
					"name": "$(CONN_CREDENTIAL_SECRET_NAME)"
				}
			}
		},
		{
			"name":  "DATA_SOURCE_NAME"
			"value": "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/"
		},
	]
	"ports": [
		{
			"containerPort": monitor.internalPort
			"name":          "scrape"
			"protocol":      "TCP"
		},
	]
	"livenessProbe": {
		"failureThreshold": 3
		"httpGet": {
			"path":   "/"
			"port":   monitor.internalPort
			"scheme": "HTTP"
		}
		"periodSeconds":    10
		"successThreshold": 1
		"timeoutSeconds":   1
	}
	"readinessProbe": {
		"failureThreshold": 3
		"httpGet": {
			"path":   "/"
			"port":   monitor.internalPort
			"scheme": "HTTP"
		}
		"periodSeconds":    10
		"successThreshold": 1
		"timeoutSeconds":   1
	}
	"resources": {}
}
