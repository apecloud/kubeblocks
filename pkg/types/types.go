/*
Copyright © 2022 The OpenCli Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

const (
	// OpenCliDefaultHome defines opencli default home name
	OpenCliDefaultHome = ".opencli"
	// OpenCliHomeEnv defines opencli home system env
	OpenCliHomeEnv = "OPENCLI_HOME"

	// K3sTokenPath is the path to k3s token
	K3sTokenPath = "/var/lib/rancher/k3s/server/token"
	// K3sKubeConfigLocation is default path of k3s kubeconfig
	K3sKubeConfigLocation = "/etc/rancher/k3s/k3s.yaml"
	// K3sExternalKubeConfigLocation is where to generate kubeconfig for external access
	K3sExternalKubeConfigLocation = "/etc/rancher/k3s/k3s-external.yaml"
	// CliDockerNetwork is docker network for k3d cluster when `opencli playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-opencli-playground"

	// K3sImage is k3s image tag
	K3sImage = "rancher/k3s:v1.21.10-k3s1"
	// K3dToolsImage is k3d tools image tag
	K3dToolsImage = "ghcr.io/k3d-io/k3d-tools:5.4.1"
	// K3dProxyImage is k3d proxy image tag
	K3dProxyImage = "ghcr.io/k3d-io/k3d-proxy:5.4.1"

	// GoosLinux is os.GOOS linux string
	GoosLinux = "linux"
	// GoosDarwin is os.GOOS darwin string
	GoosDarwin = "darwin"
	// GoosWindows is os.GOOS windows string
	GoosWindows = "windows"
)

type K3dImages struct {
	K3s      bool
	K3dTools bool
	K3dProxy bool
	Reason   string
}

// K3dStatus defines the status of k3d
type K3dStatus struct {
	Reason     string
	K3dCluster []K3dCluster
}

// K3dCluster defines the status of one k3d cluster
type K3dCluster struct {
	Name          string
	Running       bool
	ReleaseStatus map[string]string
	Reason        string
}

type ClusterStatus struct {
	K3dImages
	K3d K3dStatus
}
