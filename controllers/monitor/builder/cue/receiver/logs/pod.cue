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

parameters: {
	include: *["/var/log/pods/**/**/*.log"] | [string]
}

output: {
	"filelog/pod": {
		include:           parameters.include
		include_file_name: true
		include_file_path: true
		start_at:          "beginning"
		storage:           "file_storage/oteld"
		resource: {
			type:                   "pods"
			"loki.resource.labels": "host_name, host_ip"
			"loki.format":          "raw"
		}
		attributes:
			"loki.attribute.labels": "namespace, pod, pod_id, container, restart_num, log.file.path, log.file.name"
		retry_on_failure: {
			enabled:          true
			initial_interval: "5s"
		}
		operators: [
			{
				type:  "add"
				field: "resource.host_name"
				value: "EXPR(env(\"PWD\"))"
			},
			{
				type:  "add"
				field: "resource.host_ip"
				value: "EXPR(env(\"PWD\"))"
			},
			{
				type:       "regex_parser"
				id:         "parser_pods_logs_dir"
				parse_from: "attributes['log.file.path']"
				regex:      "^/var/log/pods/(?P<namespace>[^_]+)_(?P<pod>[^_]+)_(?P<pod_id>[^_]+)/(?P<container>[^/]+)/(?P<restart_num>\\d+).log$"
				on_error:   "send"
			},
		]
	}
}
