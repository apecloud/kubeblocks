parameters: {
	auth_type: *"service_account" | string
	collection_interval: *"10s" | string
	endpoint: *"http://localhost:10255" | string
}

output:
  apecloudkubeletstats: {
	  auth_type: parameters.auth_type
    collection_interval: parameters.collection_interval
    endpoint: parameters.endpoint
	  extra_metadata_labels:
	  	- k8s.volume.type
	  	- kubeblocks
	  metric_groups:
	  	- container
	  	- pod
  		- volume
  }


