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
	flag: int
	mode:     string
	driver:   string
	database: string
	value: 		string
}

// required, k8s api resource content
content: {
	apiVersion: "v1"
	kind:       "Pod"
	metadata: {
		namespace:    "default"
		generateName: "test-sysbench-\(options.mode)-\(options.driver)-"
		labels: {
			sysbench: "test-sysbench-\(options.driver)-\(options.database)"
		}
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
						value: "\(options.flag)"
					},
					{
						name:  "CONFIGS"
						value: "\(options.value)"
					},
				]
			},
		]
		restartPolicy: "Never"
	}
}
