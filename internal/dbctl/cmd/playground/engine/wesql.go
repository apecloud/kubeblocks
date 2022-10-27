/*
Copyright ApeCloud Inc.

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

package engine

import (
	"fmt"

	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

var (
	wesqlHelmChart = "oci://yimeisun.azurecr.io/helm-chart/wesqlcluster"
	wesqlVersion   = "0.1.0"
)

type WeSQL struct {
	name      string
	namespace string
	version   string
	replicas  int
}

func (w *WeSQL) HelmInstallOpts() *helm.InstallOpts {
	return &helm.InstallOpts{
		Name:      w.name,
		Chart:     wesqlHelmChart,
		Wait:      true,
		Namespace: w.namespace,
		Version:   wesqlVersion,
		Sets: []string{
			"serverVersion=" + w.version,
			fmt.Sprintf("replicaCount=%d", w.replicas),
		},
		Login:    true,
		TryTimes: 2,
	}
}
