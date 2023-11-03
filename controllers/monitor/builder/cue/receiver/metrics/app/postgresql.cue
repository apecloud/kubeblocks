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
	enpoint:           *"`endpoint`:5432" | string
	username:          *"`envs[\"PGUSER_SUPERUSER\"]`" | string
	password:          *"`envs[\"PGPASSWORD_SUPERUSER\"]`" | string
	job:               *"oteld-app-metrics" | string
	databases:         *["postgres"] | [string]
	exclude_databases: *["template0", "template1"] | [string]
}

output:
	apecloudpostgresql: {
		rule: "type == \"container\" && monitor_type == \"postgresql\" && config != nil && config.EnabledMetrics"
		config: {
			endpoint:            parameters.enpoint
			username:            parameters.username
			password:            parameters.password
			databases:           parameters.databases
			exclude_databases:   parameters.exclude_databases
			collection_interval: "`settings.CollectionInterval`"
			transport:           "tcp"
			tls: {
				insecure:             true
				insecure_skip_verify: true
			}
		}
		resource_attributes: {
			job:      parameters.job
			receiver: "apecloudpostgresql"
		}
	}
