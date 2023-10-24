parameters: {
	collection_interval: *"15s" | string,
	initial_delay: *"10s" | string,
	timeout: *"5s" | string,
}

output: {
	collection_interval: parameters.collection_interval,
	initial_delay: parameters.initial_delay,
	timeout: parameters.timeout,
}
