#meta: {
	endpoint: *"test" | string
	username: *"labels[\"conn_username\"]" | string
	password: *"labels[\"conn_password\"]" | string
	collection_interval: *"15s" | string
	transport: *"tcp" | string
}

output:
	apecloudmysql: {
		endpoint: parameter.endpoint
		username: parameter.username
		password: parameter.password
		collection_interval: parameter.collection_interval
		transport: parameter.transport
	}

parameter: #meta