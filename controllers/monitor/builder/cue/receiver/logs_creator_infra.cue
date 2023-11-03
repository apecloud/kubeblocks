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

output: {
	watch_observers: ["apecloud_engine_observer"]
	receivers: {
		container_filelog: {
			rule: "type == \"container\" && config != nil && config.EnabledLogs"
			config: {
				input_configs:  "`config.LogsCollector`"
				container_id:   "`container_id`"
				pod_ip:         "`endpoint`"
				storage:        "file_storage/oteld"
				cluster_name:   "`config.ClusterName`"
				component_name: "`config.ComponentName`"
				character_type: "`config.CharacterType`"
			}
		}

	}
	resource_attributes: {
		container: {
			app_kubernetes_io_component:       "`labels[\"app.kubernetes.io/component\"]`"
			app_kubernetes_io_instance:        "`labels[\"app.kubernetes.io/instance\"]`"
			app_kubernetes_io_managed_by:      "`labels[\"app.kubernetes.io/managed-by\"]`"
			app_kubernetes_io_name:            "`labels[\"app.kubernetes.io/name\"]`"
			app_kubernetes_io_version:         "`labels[\"app.kubernetes.io/version\"]`"
			apps_kubeblocks_io_component_name: "`labels[\"apps.kubeblocks.io/component-name\"]`"
			node:                              "${env:NODE_NAME}"
			namespace:                         "`namespace`"
			pod:                               "`name`"
			job:                               "oteld-app-metrics"
		}
	}
}
