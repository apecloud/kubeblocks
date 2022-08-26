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

package utils

import (
	"os"
	"text/template"

	"github.com/fatih/color"

	"jihulab.com/infracreate/dbaas-system/dbctl/pkg/types"
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
	ClusterName   string
	GrafanaSvc    string
	GrafanaPort   string
	GrafanaUser   string
	GrafanaPasswd string
	HostIP        string
	CloudProvider string
	Version       string
}

type DBClusterInfo struct {
	DBCluster       string
	DBPort          string
	Version         string
	Topology        string
	Status          string
	StartTime       string
	Labels          string
	RootUser        string
	DBNamespace     string
	Instances       int64
	ServerId        int64
	Secret          string
	OnlineInstances int64
	Storage         int64
	Engine          string
	HostIP          string
}

type BackupJobInfo struct {
	Name           string
	Namespace      string
	Phase          string
	StartTime      string
	CompletionTime string
	Labels         string
}

var playgroundTmpl = `
Notes:
Open DBaaS Playground v{{.Version}} Start SUCCESSFULLY!
MySQL Standalone Cluster "{{.DBCluster}}" has been CREATED!

1. Basic commands for dbcluster:
  dbctl --kubeconfig ~/.kube/{{.ClusterName}} dbcluster list                          # list all database clusters
  dbctl --kubeconfig ~/.kube/{{.ClusterName}} dbcluster describe {{.DBCluster}}       # get dbcluster information
  dbctl bench --host {{.HostIP}} tpcc {{.DBCluster}}                                  # run tpcc benchmark 1min on dbcluster

2. To connect to mysql database:
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/{{.ClusterName}} get secret --namespace {{.DBNamespace}} {{.DBCluster}}-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysql -h {{.HostIP}} -uroot -p"$MYSQL_ROOT_PASSWORD"

3. To view the Grafana:
  open http://{{.HostIP}}:{{.GrafanaPort}}/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql_for_demo
  User: {{.GrafanaUser}}
  Password: {{.GrafanaPasswd}}

4. Uninstall Playground:
  dbctl playground destroy

--------------------------------------------------------------------
To view this guide next time:         dbctl playground guide
To get more help information:         dbctl help
{{if ne .CloudProvider "local"}}To login to remote host:              ssh -i ~/.opendbaas/ssh/id_rsa ec2-user@{{.HostIP}}{{end}}
Use "dbctl [command] --help" for more information about a command.

`

var clusterInfoTmpl = `
Name:           {{.DBCluster}}
Kind:           {{.Engine}}
Version:        {{.Version}}
Topology mode:  {{.Topology}}
CPU:            N/A
Memory:         N/A
Storage:        {{.Storage}}Gi
Status:         {{.Status}}
Started:        {{.StartTime}}
labels:         {{.Labels}}
ServerId:       {{.ServerId}}
Endpoint:       {{.HostIP}}:{{.DBPort}}

# connect information
Username:       {{.RootUser}}
Password:       MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/dbctl-playground get secret --namespace {{.DBNamespace}} {{.DBCluster}}-cluster-secret -o jsonpath="{.data.rootPassword}" | base64 -d)
Connect:        mysql -h {{.HostIP}} -u{{.RootUser}} -p"$MYSQL_ROOT_PASSWORD"

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
	return PrintTemplate(playgroundTmpl, info)
}

func PrintClusterInfo(info *DBClusterInfo) error {
	return PrintTemplate(clusterInfoTmpl, info)
}

func PrintTemplate(t string, data interface{}) error {
	tmpl, err := template.New("_").Parse(t)
	if err != nil {
		return err
	}

	err = tmpl.Execute(os.Stdout, data)
	if err != nil {
		return err
	}

	return nil
}
