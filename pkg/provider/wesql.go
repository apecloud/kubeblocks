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

package provider

import (
	"helm.sh/helm/v3/pkg/repo"

	"github.com/apecloud/kubeblocks/pkg/utils/helm"
)

type Wesql struct {
	serverVersion string
}

func (o *Wesql) GetRepos() []repo.Entry {
	return []repo.Entry{}
}

func (o *Wesql) GetBaseCharts(ns string) []helm.InstallOpts {
	return []helm.InstallOpts{}
}

func (o *Wesql) GetDBCharts(ns string, dbname string) []helm.InstallOpts {
	return []helm.InstallOpts{
		{
			Name:      "opendbaas-core",
			Chart:     "oci://yimeisun.azurecr.io/helm-chart/opendbaas-core",
			Wait:      true,
			Version:   "0.1.0-alpha.3",
			Namespace: "default",
			Sets:      []string{},
			LoginOpts: &helm.LoginOpts{
				User:   helmUser,
				Passwd: helmPasswd,
				URL:    helmURL,
			},
			TryTimes: 2,
		},
		{
			Name:      dbname,
			Chart:     "oci://yimeisun.azurecr.io/helm-chart/wesqlcluster",
			Wait:      true,
			Namespace: "default",
			Version:   "0.1.0",
			Sets: []string{
				"serverVersion=" + o.serverVersion,
			},
			LoginOpts: &helm.LoginOpts{
				User:   helmUser,
				Passwd: helmPasswd,
				URL:    helmURL,
			},
			TryTimes: 2,
		},
	}
}
