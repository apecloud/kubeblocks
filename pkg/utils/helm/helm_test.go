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

package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/repo"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/utils"
)

func TestAddRepo(t *testing.T) {
	r := repo.Entry{
		Name: "mysql-operator",
		URL:  "https://mysql.github.io/mysql-operator/",
	}
	//nolint
	AddRepo(&r)
}

func TestInstall(t *testing.T) {
	is := assert.New(t)
	installs := []InstallOpts{
		{
			Name:      "my",
			Chart:     "oci://yimeisun.azurecr.io/helm-chart/mysql-innodbcluster",
			Wait:      true,
			Namespace: "default",
			Version:   "1.0.0",
			Sets:      []string{},
			LoginOpts: &LoginOpts{
				User:   "yimeisun",
				Passwd: "8V+PmX1oSDv4pumDvZp6m7LS8iPgbY3A",
				URL:    "yimeisun.azurecr.io",
			},
		},
	}

	for _, i := range installs {
		res, err := i.Install(utils.ConfigPath("opencli-playground"))
		is.Equal(err, nil)
		is.Equal(res.Name, i.Name)
	}
}
