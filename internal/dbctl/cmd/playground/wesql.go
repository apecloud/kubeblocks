/*
Copyright © 2022 The dbctl Authors

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
	"fmt"

	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
)

type Wesql struct {
	serverVersion string
	replicas      int8
}

func (o *Wesql) getRepos() []repo.Entry {
	return []repo.Entry{}
}

func (o *Wesql) getBaseCharts(ns string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      types.DbaasHelmName,
			Chart:     types.DbaasHelmChart,
			Wait:      true,
			Version:   types.DbaasDefaultVersion,
			Namespace: "default",
			Sets: []string{
				"image.tag=latest",
				"image.pullPolicy=Always",
			},
			Login:    true,
			TryTimes: 2,
		},
	}
}

func (o *Wesql) getDBCharts(ns string, dbname string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      dbname,
			Chart:     wesqlHelmChart,
			Wait:      true,
			Namespace: "default",
			Version:   wesqlVersion,
			Sets: []string{
				"serverVersion=" + o.serverVersion,
				fmt.Sprintf("replicaCount=%d", o.replicas),
			},
			Login:    true,
			TryTimes: 2,
		},
	}
}
