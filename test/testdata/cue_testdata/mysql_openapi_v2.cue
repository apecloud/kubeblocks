#SectionParameter: {
	// [OFF|ON] default ON
	automatic_sp_privileges: string & "OFF" | "ON" | *"ON"
	// [1~65535] default ON
	auto_increment_increment: int & >=1 & <=65535 | *1
	binlog_stmt_cache_size:   int & >=4096 & <=16777216 | *2097152
	// [0|1|2] default: 2
	innodb_autoinc_lock_mode?: int & 0 & 1 & 2 | *2
	flush_time:                int & >=0 & <=31536000 | *0
	group_concat_max_len:      int & >4 | *1024

	gtid_mode: string & "0" | "1" | "OFF" | "ON"

	port: int

	// [0|1] default empty
	innodb_buffer_pool_load_now?: int & 0 | 1

	// custom format, reference Regular expressions
	KeyPath:                                 =~"^[a-z][a-zA-Z0-9]+.pem$"
	caching_sha2_password_private_key_path?: KeyPath
}

// mysql all parameters definition
#MysqlSchema: {

	// mysql config validator
	mysqlld: #SectionParameter

	// ingore client parameter validate
	// mysql client: a set of name/value pairs.
	client?: {
		[string]: string
	} @protobuf(2,type=map<string,string>)
}

[SectionName=_]: #SectionParameter
