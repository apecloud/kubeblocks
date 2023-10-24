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

output:
  apecloudmongo: {
  	rule: "type == \"container\" && monitor_type == \"mongodb\" && config != nil && config.EnabledMetrics"
		config: {
			endpoint: "`endpoint`:27017"
      username: "`envs[\"MONGODB_ROOT_USER\"]`"
      password: "`envs[\"MONGODB_ROOT_PASSWORD\"]`"
      connect_params: "admin?ssl=false&authSource=admin"
      collect_all: true
      collection_interval: "`settings.CollectionInterval`"
      direct_connect: true
      global_conn_pool: false
      compatible_mode: true
		}
		resource_attributes: {
		  job: "oteld-app-metrics"
      receiver: "apecloudmongodb"
		}
  }