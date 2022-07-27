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
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/repo"

	"testing"

	"github.com/infracreate/opencli/pkg/utils"
)

func TestAddRepo(t *testing.T) {
	r := repo.Entry{
		Name: "mysql-operator",
		URL:  "https://mysql.github.io/mysql-operator/",
	}
	AddRepo(&r)
}

func TestInstall(t *testing.T) {
	is := assert.New(t)
	installs := []InstallOpts{
		{
			Name:      "my-mysql-operator",
			Chart:     "mysql-operator/mysql-operator",
			Namespace: "mysql-operator",
			Sets:      []string{},
		},
		{
			Name:      "mycluster",
			Chart:     "mysql-operator/mysql-innodbcluster",
			Namespace: "mysql-operator",
			Sets: []string{"credentials.root.user='root'",
				"credentials.root.password='sakila'",
				"credentials.root.host='%'",
				"serverInstances=1",
				"routerInstances=1",
				"tls.useSelfSigned=true",
			},
		},
	}

	for _, i := range installs {
		res, err := i.Install(utils.ConfigPath("config"))
		is.Equal(err, nil)
		is.Equal(res.Name, i.Name)
	}
}
