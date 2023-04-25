// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

template: {
	name: parameter.name
	command: [
		"env",
	]
	args: [
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$(TOOLS_PATH)",
		"/bin/reloader",
		for arg in parameter.args {
			arg
		},
	]
	env: [
		{
			name: "CONFIG_MANAGER_POD_IP"
			valueFrom:
				fieldRef:
					fieldPath: "status.podIP"
		},
		if parameter.characterType != "" {
			{
				name:  "DB_TYPE"
				value: parameter.characterType
			}
		},
		if parameter.characterType == "mysql" {
			{
				name: "MYSQL_USER"
				valueFrom: {
					secretKeyRef: {
						key:  "username"
						name: parameter.secreteName
					}
				}
			}
		},
		if parameter.characterType == "mysql" {
			{
				name: "MYSQL_PASSWORD"
				valueFrom: {
					secretKeyRef: {
						key:  "password"
						name: parameter.secreteName
					}
				}
			}
		},
		if parameter.characterType == "mysql" {
			{
				name:  "DATA_SOURCE_NAME"
				value: "$(MYSQL_USER):$(MYSQL_PASSWORD)@(localhost:3306)/"
			}
		},
		// other type
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
	name:          string
	characterType: string
	sidecarImage:  string
	secreteName:   string
	args: [...#ArgType]
	// envs?: [...#EnvType]
	volumes: [...]
}
