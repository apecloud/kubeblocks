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

package playground

import (
	"github.com/fatih/color"
	"github.com/infracreate/opencli/pkg/types"
	"github.com/infracreate/opencli/pkg/utils"
)

var (
	red            = color.New(color.FgRed).SprintFunc()
	green          = color.New(color.FgGreen).SprintFunc()
	k3dImageStatus = map[string]bool{}
	x              = red("✘")
	y              = green("✔")
)

func printClusterStatusK3d(status types.ClusterStatus) bool {
	utils.InfoP(0, "K3d images status:")
	if status.K3dImages.Reason != "" {
		utils.Info(x, "K3d images:", status.K3dImages.Reason)
		return true // k3d images not ready
	}
	k3dImageStatus[types.K3sImage] = status.K3dImages.K3s
	k3dImageStatus[types.K3dToolsImage] = status.K3dImages.K3dTools
	k3dImageStatus[types.K3dProxyImage] = status.K3dImages.K3dProxy
	stop := false
	for i, imageStatus := range k3dImageStatus {
		stop = stop || !imageStatus
		if !imageStatus {
			utils.InfoP(1, x, "image", i, "not ready")
		} else {
			utils.InfoP(1, y, "image", i, "ready")
		}
	}
	if stop {
		return stop
	}
	utils.InfoP(0, "Cluster(K3d) status:")
	if status.K3d.Reason != "" {
		utils.Info(x, "K3d:", status.K3d.Reason)
		return true // k3d not ready
	}
	for _, c := range status.K3d.K3dCluster {
		cr := x
		if c.Reason != "" {
			utils.InfoP(1, x, "cluster", "[", c.Name, "]", "not ready:", c.Reason)
			stop = true
		} else {
			utils.InfoP(1, y, "cluster", "[", c.Name, "]", "ready")
			cr = y
		}

		// Print helm release status
		for k, v := range c.ReleaseStatus {
			utils.InfoP(2, cr, "helm chart [", k, "] status:", v)
		}
	}
	if stop {
		return stop
	}

	return false
}
