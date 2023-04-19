// Copyright ApeCloud, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	// ignore client parameter validate
	// mysql client: a set of name/value pairs.
	client?: {
		[string]: string
	} @protobuf(2,type=map<string,string>)
}

[SectionName=_]: #SectionParameter
