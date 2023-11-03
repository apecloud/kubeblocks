logsInclude: {
	[string]: {
		include: [...string]
	}
}

parameters: {
	cluster_name:   string
	component_name: string
	container_name: string
	collector_name: string
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
	"\(parameters.cluster_name)/\(parameters.component_name)/\(parameters.container_name)/\(parameters.collector_name)": {
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
