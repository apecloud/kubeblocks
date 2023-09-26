parameters: {
	collection_interval: *"15s" | string
}


output:
	k8s_cluster: {
		collection_interval: parameters.collection_interval
	}

