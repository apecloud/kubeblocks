// top level configuration type
//  mysql server param: a set of name/value pairs.
#MysqlParameter: {
	// [OFF|ON] default ON
	automatic_sp_privileges: string & "OFF" | "ON" | *"ON"
	// [1~65535] default ON
	auto_increment_increment: int & >=1 & <=65535 | *1
	// [4096~16777216] default 2G
	binlog_stmt_cache_size?: int & >=4096 & <=16777216 | *2097152
	// [0|1|2] default: 2
	innodb_autoinc_lock_mode?: int & 0 | 1 | 2 | *2
	// other parmeters
	// reference mysql parmeters
	...
}
mysqld: #MysqlParameter & {
}
// ingore client parameter validate
// mysql client: a set of name/value pairs.
client?: {
	[string]: string
} @protobuf(2,type=map<string,string>)
