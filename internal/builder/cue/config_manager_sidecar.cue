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
