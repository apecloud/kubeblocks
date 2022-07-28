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

package utils

import (
	"os"
	"text/template"

	"github.com/fatih/color"

	"jihulab.com/infracreate/dbaas-system/opencli/pkg/types"
)

var (
	red            = color.New(color.FgRed).SprintFunc()
	green          = color.New(color.FgGreen).SprintFunc()
	k3dImageStatus = map[string]bool{}
	x              = red("✘")
	y              = green("✔")
)

type PlayGroundInfo struct {
	DBCluster     string
	DBPort        string
	DBNamespace   string
	Namespace     string
	GrafanaSvc    string
	GrafanaPort   string
	GrafanaUser   string
	GrafanaPasswd string
}

var playgroundTmpl = `
Notes:
** Please be patient while playground is being deployed **
DBaaS playground v0.1.0 Start SUCCESSFULLY!

To view the db clusters by command client:
  opencli dbcluster list

Execute the following command in another terminal
  kubectl port-forward --address 0.0.0.0 service/{{.DBCluster}} {{.DBPort}}
  kubectl port-forward --namespace {{.Namespace}} svc/{{.GrafanaSvc}} {{.GrafanaPort}}:80

To view the Grafana: http://127.0.0.1:{{.GrafanaPort}}   {{.GrafanaUser}}/{{.GrafanaPasswd}}

** MySQL cluster {{.DBCluster}} is being created **
Execute the following to get the administrator credentials:

  MYSQL_ROOT_PASSWORD=$(kubectl get secret --namespace {{.DBNamespace}} {{.DBCluster}}-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)

To connect to your database:

  1. To connect to primary service (read/write):

      mysql -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD"


  2. To connect to primary service(read/write) by JDBC:

      jdbc:mysql://127.0.0.1:{{.DBPort}}/mysql
`

func PrintClusterStatus(status types.ClusterStatus) bool {
	InfoP(0, "K3d images status:")
	if status.K3dImages.Reason != "" {
		Info(x, "K3d images:", status.K3dImages.Reason)
		return true // k3d images not ready
	}
	k3dImageStatus[types.K3sImage] = status.K3dImages.K3s
	k3dImageStatus[types.K3dToolsImage] = status.K3dImages.K3dTools
	k3dImageStatus[types.K3dProxyImage] = status.K3dImages.K3dProxy
	stop := false
	for i, imageStatus := range k3dImageStatus {
		stop = stop || !imageStatus
		if !imageStatus {
			InfoP(1, x, "image", i, "not ready")
		} else {
			InfoP(1, y, "image", i, "ready")
		}
	}
	if stop {
		return stop
	}
	InfoP(0, "Cluster(K3d) status:")
	if status.K3d.Reason != "" {
		Info(x, "K3d:", status.K3d.Reason)
		return true // k3d not ready
	}
	for _, c := range status.K3d.K3dCluster {
		cr := x
		if c.Reason != "" {
			InfoP(1, x, "cluster", "[", c.Name, "]", "not ready:", c.Reason)
			stop = true
		} else {
			InfoP(1, y, "cluster", "[", c.Name, "]", "ready")
			cr = y
		}

		// Print helm release status
		for k, v := range c.ReleaseStatus {
			InfoP(2, cr, "helm chart [", k, "] status:", v)
		}
	}
	if stop {
		return stop
	}

	return false
}

func PrintPlaygroundGuild(info PlayGroundInfo) error {
	tmpl, err := template.New("test").Parse(playgroundTmpl)
	if err != nil {
		return err
	}

	err = tmpl.Execute(os.Stdout, info)
	if err != nil {
		return err
	}

	return nil
}
