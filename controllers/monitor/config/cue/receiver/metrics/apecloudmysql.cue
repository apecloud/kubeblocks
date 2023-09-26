parameters: {
	username: *"${env:MYSQL_USER}" | string
  password: *"${env:MYSQL_PASSWORD}" | string
  allow_native_passwords: *true | bool
  transport: *"tcp" | string
  collection_interval: *"${env:COLLECTION_INTERVAL}" | string
}



output:
  apecloudmysql: {
    username: parameters.username
    password: parameters.password
    allow_native_passwords: parameters.allow_native_passwords
    transport: parameters.transport
    collection_interval: parameters.collection_interval
  }

