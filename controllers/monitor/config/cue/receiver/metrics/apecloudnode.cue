parameters: {
	collection_interval: *"15s" | string
}


output:
  apecloudnode: {
  	collection_interval: parameters.collection_interval
  }

