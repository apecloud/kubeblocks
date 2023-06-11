//Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
//This file is part of KubeBlocks project
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published by
//the Free Software Foundation, either version 3 of the License, or
//(at your option) any later version.
//
//This program is distributed in the hope that it will be useful
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <http://www.gnu.org/licenses/>.

// required, command line input options for parameters and flags
options: {
	driver:   string
	database: string
	host:     string
	port:     int
	user:     string
	password: string
	mode:     string
	type:     string
	tables:   int
	times:    int
}

// required, k8s api resource content
content: {
	apiVersion: "v1"
	kind:       "Pod"
	metadata: {
		namespace:    "default"
		generateName: "test-sysbench-prepare-\(options.driver)-"
	}
	spec: {
		containers: [
			{
				name:  "test-sysbench"
				image: "registry.cn-hangzhou.aliyuncs.com/apecloud/customsuites:latest"
				env: [
					{
						name:  "TYPE"
						value: "2"
					},
					{
						name:  "FLAG"
						value: "0"
					},
					{
						name:  "CONFIGS"
						value: "mode:\(options.mode),driver:\(options.driver),host:\(options.host),user:\(options.user),password:\(options.password),port:\(options.port),db:\(options.database),size:\(options.size),tables:\(options.tables),times:\(options.times),type:\(options.type)"
					},
				]
			},
		]
		restartPolicy: "Never"
	}
}
