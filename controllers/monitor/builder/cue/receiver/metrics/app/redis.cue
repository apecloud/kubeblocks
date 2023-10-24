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
	job: *"oteld-app-metrics" | string
	endpoint: *"`endpoint`:6379" | string
	username: *"envs[\"REDIS_REPL_USER\"]" | string
	password: *"envs[\"REDIS_REPL_PASSWORD\"]" | string
}

output:
  apecloudredis: {
  	rule: "type == \"container\" && monitor_type == \"redis\" && config != nil && config.EnabledMetrics"
		config: {
			endpoint: parameters.endpoint
			username: parameters.username
			password: parameters.password
			password_file: ""
			lua_script: ""
			tls: {
				insecure: true
				insecure_skip_verify: true
			}
			collection_interval: "`settings.CollectionInterval`"
		}

		resource_attributes: {
			job: parameters.job
			receiver: "apecloudredis"
		}
  }