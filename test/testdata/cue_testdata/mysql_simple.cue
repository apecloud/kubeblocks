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

#Section: {
	// SectionName is extract section name

	// [OFF|ON] default ON
	automatic_sp_privileges: string & "OFF" | "ON" | *"ON"

	// [1~65535] default ON
	auto_increment_increment: int & >=1 & <=65535 | *1

	binlog_stmt_cache_size?: int & >=4096 & <=16777216 | *2097152
	// [0|1|2] default: 2
	innodb_autoinc_lock_mode?: int & 0 | 1 | 2 | *2

	// other parameters
	// reference mysql parameters
	...
}

[SectionName=_]: #Section
