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

probeContainer: {
	image: "registry.cn-hangzhou.aliyuncs.com/google_containers/pause:3.6"
	command: ["/pause"]
	imagePullPolicy: "IfNotPresent"
	name:            "string"
	"env": [
		{
			"name": "KB_SERVICE_USER"
			"valueFrom": {
				"secretKeyRef": {
					"key":  "username"
					"name": "$(CONN_CREDENTIAL_SECRET_NAME)"
				}
			}
		},
		{
			"name": "KB_SERVICE_PASSWORD"
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
