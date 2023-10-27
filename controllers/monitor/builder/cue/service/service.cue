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
	log_level: *"debug" | string
  metrics_port: *6668 | int
}

output: {
	telemetry: {
  	logs:
      level: parameters.logLevel
    metrics:
      address: "${env:HOST_IP}:" + "\(parameters.metricsPort)"
    resource: {
    	node: "${env:NODE_NAME}"
      job: "oteld-telemetry"
    }
  }
  extensions: ["memory_ballast", "apecloud_k8s_observer", "runtime_container", "apecloud_engine_observer", "file_storage/oteld"]
}
