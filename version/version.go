/*
Copyright 2022 The KubeBlocks Authors

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

package version

// Version is the string that contains version
var Version = "edge"

// BuildDate is the string of binary build date
var BuildDate string

// GitCommit is the string of git commit ID
var GitCommit string

// GitVersion is the string of git version tag
var GitVersion string

// K3dVersion is k3d release version
var K3dVersion = "5.4.4"

// K3sImageTag is k3s image tag
var K3sImageTag = "v1.23.8-k3s1"

// DefaultKubeBlocksVersion the default KubeBlocks version that dbctl installed
var DefaultKubeBlocksVersion string

// GetVersion returns the version for cli, it gets it from "git describe --tags" or returns "dev" when doing simple go build
func GetVersion() string {
	if len(Version) == 0 {
		return "v1-dev"
	}
	return Version
}
