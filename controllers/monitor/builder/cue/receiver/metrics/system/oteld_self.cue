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
	collection_interval: *"15s" | string
	metrics_port:        *6668 | int
	job:                 *"oteld-self-metrics" | string
}

output:
	"prometheus/self-metrics": config: scrape_configs: [{
		job_name:        parameters.job
		scrape_interval: parameters.collection_interval
		static_configs: [{
			targets: ["${env:HOST_IP}:" + "\(parameters.metrics_port)"]
		}]
	}]
