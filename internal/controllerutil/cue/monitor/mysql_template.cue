monitor: {
	secretName:   string
	internalPort: int
}

container: {
	"name":            "inject-mysql-exporter"
	"imagePullPolicy": "IfNotPresent"
	"image":           "prom/mysqld-exporter:v0.14.0"
	"env": [
		{
			"name": "MYSQL_USER"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "username"
					"name": "\(monitor.secretName)"
				}
			}
		},
		{
			"name": "MYSQL_PASSWORD"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "password"
					"name": "\(monitor.secretName)"
				}
			}
		},
		{
			"name":  "DATA_SOURCE_NAME"
			"value": "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/"
		}
	]
	"ports": [
		{
			"containerPort": monitor.internalPort
			"name":          "scrape"
			"protocol":      "TCP"
		}
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
