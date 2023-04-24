// Copyright (C) 2022-2023 ApeCloud Co., Ltd
//
// This file is part of KubeBlocks project
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// mysql config validator
//  mysql server param: a set of name/value pairs.
mysqld: {
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

// ignore client parameter validate
// mysql client: a set of name/value pairs.
client?: {
	[string]: string
} @protobuf(2,type=map<string,string>)
