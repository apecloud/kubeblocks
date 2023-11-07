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

logsInclude: {
	[string]: {
		include: [...string]
	}
}

parameters: {
	cluster_name?:      string
	component_name?:    string
	container_name:     string
	collector_name:     string
	cluster_def_name:   string
	component_def_name: string
	metrics: {
		enabled:             *true | bool
		collection_interval: *"30s" | string
		enabled_metrics: [...string]
	}
	logs: {
		enabled:         *true | bool
		logs_collector?: _|_ | logsInclude
	}
}

output: {
	"\(parameters.cluster_def_name)/\(parameters.component_def_name)/\(parameters.container_name)/\(parameters.collector_name)": {
		enabled_metrics: parameters.metrics.enabled
		enabled_logs:    parameters.logs.enabled
		metrics_collector: {
			collection_interval: parameters.metrics.collection_interval
			if parameters.enabled_metrics != _|_ {
				enable_metrics?: parameters.enabled_metrics
			}
		}
		if parameters.logs.logs_collector != _|_ {
			logs_collector: {
				for k, v in parameters.logs.logs_collector {
					"\(k)": {
						include: v.include
					}
				}
			}
		}
		if parameters.extra_labels != _|_ {
			external_labels: parameters.extra_labels
		}
	}
}
