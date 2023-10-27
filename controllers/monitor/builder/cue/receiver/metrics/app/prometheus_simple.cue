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
	metrics_path: *"/metrics" | string,
	endpoint: *"`endpoint`:`envs[\"VTTABLET_PORT\"]`" | string,
	disable_keep_alives: *false | bool,
}

output:
	"prometheus_simple": {
		rule: "type == \"container\" && monitor_type == \"prometheus\" && config != nil && config.EnabledMetrics"
    	config: {
    		metrics_path: "`config.Prometheus == nil ? \"/metrics\" : config.Prometheus.MetricsPath`"
        endpoint: "`endpoint`:`envs[\"SERVICE_PORT\"]`"
        disable_keep_alives: "`config.Prometheus == nil ? false : config.Prometheus.DisableKeepAlives`"
        use_service_account: "`config.Prometheus == nil ? false : config.Prometheus.UseServiceAccount`"
    	}
	}
