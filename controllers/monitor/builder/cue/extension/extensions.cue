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
	enable: *true | bool
  container_runtime_type: *"containerd" | string
}

output: {
	extensions: {
  	memory_ballast:
      size_mib: 32
    apecloud_k8s_observer: {
    	auth_type: "kubeConfig"
      node: "${env:NODE_NAME}"
      observe_pods: true
      observe_nodes: false
    }
    "apecloud_k8s_observer/node": {
    	auth_type: "kubeConfig"
      node: "${env:NODE_NAME}"
      observe_pods: false
      observe_nodes: true
    }
    runtime_container: {
    	enable: true
      auth_type: "kubeConfig"
      kubernetes_node: "${env:NODE_NAME}"
    }
    apecloud_engine_observer: {
    	pod_observer: "apecloud_k8s_observer"
      container_observer: "runtime_container"
      scraper_config_file: "/tmp/oteld_test_work/kb_engine.yaml"
    }
  }
}



