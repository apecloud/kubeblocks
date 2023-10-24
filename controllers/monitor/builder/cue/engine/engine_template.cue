logs: {
	include:[string]
}

parameters: {
	clusterName: string
	componentName: string
	containerName: string
	metrics: {
		enabled: *true | bool
		collectionInterval: *"30s" | string
		enabled_metrics?: [string]
	}
	logs: {
		enabled: *true | bool
		logs_collector?: [logs]
	}
}

output: {
	"\(parameters.clusterName)/\(parameters.componentName)/\(parameters.containerName)/\(parameters.containerName)": {
		enabled_metrics: parameters.metrics.enabled
		enabled_logs: parameters.logs.enabled
		metrics_collector: {
			collection_interval: parameters.collection_interval
			if parameters.enabled_metrics != _|_ {
				enable_metrics: parameters.enabled_metrics
			}
		}
		if logs.logs_collector != _|_ {
			logs_collector: logs.logs_collector
		}
	}
}