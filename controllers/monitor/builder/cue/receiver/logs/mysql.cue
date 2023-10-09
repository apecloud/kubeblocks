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

parameters: {
	slow: {
		type: *"slow" | string
		include: *[ "/data/mysql/log/mysqld-slowquery.log"] | [...string]
		include_file_name: *false | bool
		start_at: *"beginning" | bool
	}

  error: {
  	type: *"error" | string
		include: *[ "/data/mysql/log/mysqld-error.log"] | [...string]
		include_file_name: *false | bool
		start_at: *"beginning" | bool
  }
}


output: {
	"filelog/mysql/error": {
		type: parameters.error.type
		include: parameters.error.include
		include_file_name: parameters.error.include_file_name
		start_at: parameters.error.start_at
	}

	"filelog/mysql/slow":{
		type: parameters.slow.type
		include: parameters.slow.include
		include_file_name: parameters.slow.include_file_name
		start_at: parameters.slow.start_at
	}
}
