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

package resources

import "embed"

var (
	// K3sBinaryLocation is where to save k3s binary
	K3sBinaryLocation = "/usr/local/bin/k3s"
	// K3sImageDir is the directory to save the k3s air-gap image
	K3sImageDir = "/var/lib/rancher/k3s/agent/images/"
	// K3sImageLocation is where to save k3s air-gap images
	K3sImageLocation = "/var/lib/rancher/k3s/agent/images/k3s-airgap-images.tar.gz"
)

var (
	//go:embed static/k3s/images
	// K3sImage see static/k3s/images
	K3sImage embed.FS
)
