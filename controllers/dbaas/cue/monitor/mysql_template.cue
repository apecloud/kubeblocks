// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"command": ["/bin/agamotto", "--collect.info_schema.wesql_consensus"]
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
