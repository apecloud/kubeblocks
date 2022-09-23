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
	"os"
	"text/template"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var playgroundTmpl = `
Notes:
Open DBaaS Playground v{{.Version}} Start SUCCESSFULLY!
MySQL X-Cluster(WeSQL) "{{.DBCluster}}" has been CREATED!

1. Basic commands for cluster:
  dbctl --kubeconfig ~/.kube/{{.ClusterName}} cluster list                          # list all database clusters
  dbctl --kubeconfig ~/.kube/{{.ClusterName}} cluster describe {{.DBCluster}}       # get cluster information
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/{{.ClusterName}} get secret \
	--namespace {{.DBNamespace}} {{.DBCluster}} \
	-o jsonpath="{.data.rootPassword}" | base64 -d) 
  dbctl bench --host {{.HostIP}} --port $MYSQL_PRIMARY_0 --password "$MYSQL_ROOT_PASSWORD" tpcc prepare|run|clean   # run tpcc benchmark 1min on cluster

2. To port forward
  MYSQL_PRIMARY_0=3306
  MYSQL_PRIMARY_1=3307
  MYSQL_PRIMARY_2=3308
  kubectl --kubeconfig ~/.kube/{{.ClusterName}} port-forward \
  	--address 0.0.0.0 svc/{{.DBCluster}}-replicasets-primary-0 $MYSQL_PRIMARY_0:3306
  kubectl --kubeconfig ~/.kube/{{.ClusterName}} port-forward \
  	--address 0.0.0.0 svc/{{.DBCluster}}-replicasets-primary-1 $MYSQL_PRIMARY_1:3306
  kubectl --kubeconfig ~/.kube/{{.ClusterName}} port-forward \
  	--address 0.0.0.0 svc/{{.DBCluster}}-replicasets-primary-2 $MYSQL_PRIMARY_2:3306

3. To connect to mysql database:
  Assume WeSQL leader node is {{.DBCluster}}-replicasets-primary-0. 
  In practice, we can get cluster node role by sql " select * from information_schema.wesql_cluster_local; ".
  
  MYSQL_ROOT_PASSWORD=$(kubectl --kubeconfig ~/.kube/{{.ClusterName}} get secret --namespace {{.DBNamespace}} {{.DBCluster}} -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysql -h {{.HostIP}} -uroot -p"$MYSQL_ROOT_PASSWORD" -P$MYSQL_PRIMARY_0
  
4. To view the Grafana:
  open http://{{.HostIP}}:{{.GrafanaPort}}/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql_for_demo
  User: {{.GrafanaUser}}
  Password: {{.GrafanaPasswd}}

5. Uninstall Playground:
  dbctl playground destroy

--------------------------------------------------------------------
To view this guide next time:         dbctl playground guide
To get more help information:         dbctl help
{{if ne .CloudProvider "local"}}To login to remote host:              ssh -i ~/.opendbaas/ssh/id_rsa ec2-user@{{.HostIP}}{{end}}
Use "dbctl [command] --help" for more information about a command.

`

func printPlaygroundGuide(info types.PlaygroundInfo) error {
	return printTemplate(playgroundTmpl, info)
}

func printTemplate(t string, data interface{}) error {
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
