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
	name:                                     *"default" | string
	endpoint:                                 *"${env:HOST_IP}:1234" | string
	send_timestamps:                          *false | bool
	metric_expiration:                        *"20s" | string
	enable_open_metrics:                      *false | bool
	resource_to_telemetry_conversion_enabled: *true | bool
}

output: {
	"prometheus/\(parameters.name)": {
		endpoint:            parameters.endpoint
		send_timestamps:     parameters.send_timestamps
		metric_expiration:   parameters.metric_expiration
		enable_open_metrics: parameters.enable_open_metrics
		resource_to_telemetry_conversion:
			enabled: parameters.resource_to_telemetry_conversion_enabled
	}
}
