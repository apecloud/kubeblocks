/*
Copyright (C) 2022 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

// DefaultKubeBlocksVersion the default KubeBlocks version that kbcli installed
var DefaultKubeBlocksVersion string

// GetVersion returns the version for cli, it gets it from "git describe --tags" or returns "dev" when doing simple go build
func GetVersion() string {
	if len(Version) == 0 {
		return "v1-dev"
	}
	return Version
}
