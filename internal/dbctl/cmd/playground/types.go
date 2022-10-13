/*
Copyright 2022 The KubeBlocks Authors

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
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	dbaasv1alpha1 "github.com/apecloud/kubeblocks/apis/dbaas/v1alpha1"
	"github.com/apecloud/kubeblocks/version"
)

const (
	defaultCloudProvider = "local"
	DefaultVersion       = "8.0.28"
	localHost            = "127.0.0.1"
	defaultReplicas      = 3

	// CliDockerNetwork is docker network for k3d cluster when `dbctl playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-dbctl-playground"

	wesqlHelmChart = "oci://yimeisun.azurecr.io/helm-chart/wesqlcluster"
	wesqlVersion   = "0.1.0"
)

var (
	// dbClusterName is the playground database cluster name
	dbClusterName = "mycluster"
	// dbClusterNamespace is the namespace of playground database cluster
	dbClusterNamespace = "default"
	// clusterName is the k3d cluster name for playground
	clusterName = "dbctl-playground"

	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag
	// K3dToolsImage is k3d tools image repo
	K3dToolsImage = "docker.io/infracreate/k3d-tools:" + version.K3dVersion
	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/infracreate/k3d-proxy:" + version.K3dVersion
)

type ClusterInfo struct {
	Cluster      *dbaasv1alpha1.Cluster
	StatefulSets []appv1.StatefulSet
	Deployments  []appv1.Deployment
	Pods         []corev1.Pod
	Services     []corev1.Service
	Secrets      []corev1.Secret

	HostPorts     []string
	HostIP        string
	KubeConfig    string
	CloudProvider string

	GrafanaPort   string
	GrafanaUser   string
	GrafanaPasswd string
}

var kubeConfig = `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://${KUBERNETES_API_SERVER_ADDRESS}:6444
  name: k3d-dbctl-playground
contexts:
- context:
    cluster: k3d-dbctl-playground
    user: admin@k3d-dbctl-playground
  name: k3d-dbctl-playground
current-context: k3d-dbctl-playground
kind: Config
preferences: {}
users:
- name: admin@k3d-dbctl-playground
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrRENDQVRlZ0F3SUJBZ0lJR1dEc0wyWmFtWjB3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOalU1TlRnek1EUTBNQjRYRFRJeU1EZ3dOREF6TVRjeU5Gb1hEVEl6TURndwpOREF6TVRjeU5Gb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJKbkxHR1FNUmZva2srWDcKSS9HNWRSbG5sUzYwODlqWGV3Q0l1OGVvNmc5bUVlU203NWRmdzc2R2IrZ29BbXFXK244MkNqRVd1QTNrSEQyeQpQTUxSS2JhalNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCU1Fhd1VYVEZjMzVCdWJkQkdrK3ExZXZ4VW5SVEFLQmdncWhrak9QUVFEQWdOSEFEQkUKQWlBVXl0dWxOQzVVbnRCcmlvOGlhd1gxUUdjTEVxUENPWk04VmFETXozMTBoUUlnTWIxSHJGa3JXUHFWSTVvQgpBdyttN2szK0I5SzBWem1mcTJtSmx3V2pNdmM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUyTlRrMU9ETXdORFF3SGhjTk1qSXdPREEwTURNeE56STBXaGNOTXpJd09EQXhNRE14TnpJMApXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUyTlRrMU9ETXdORFF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFRUWF0NDNGSFl0ZlpyT2YreHZwaFhacUEvaEFSTUhFd2JBcDBGSVdzTUcKMmlGVnZCbThBWE9MUWxYY0VKSW5EVmppZjFZYkFISWhiYVl2WjY4NXk0SzNvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWtHc0ZGMHhYTitRYm0zUVJwUHF0ClhyOFZKMFV3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQUxZUU1qMkRqbnNRd2lKUGd0UlE3d3VDN1piMDd1VzEKZXU2SDhoaFBCN2l4QWlCbkJmQlU3M3BkSWFCdVBxNGR2TGw1MDloTWNtU1FXTVo4VVpoV1lPS0FNUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUM1VUgzOC91VXJVQWJZbENnSTZmU25kTEhVUi9lNFJ4L3JQNkdUMUNoeXRvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFbWNzWVpBeEYraVNUNWZzajhibDFHV2VWTHJUejJOZDdBSWk3eDZqcUQyWVI1S2J2bDEvRAp2b1p2NkNnQ2FwYjZmellLTVJhNERlUWNQYkk4d3RFcHRnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`

var guideTmpl = `
KubeBlocks playground init SUCCESSFULLY!
MySQL X-Cluster(WeSQL) "{{.Cluster.Name}}" has been CREATED!

1. Basic commands for cluster:

  export KUBECONFIG={{.KubeConfig}}

  dbctl cluster list                     # list all database clusters
  dbctl cluster describe {{.Cluster.Name}}       # get cluster information

2. To port forward
{{range $i, $t := .HostPorts}}
  MYSQL_PRIMARY_{{$i}}={{.}} {{end}}
{{range $i, $t := .HostPorts}}
  kubectl port-forward --address 0.0.0.0 svc/{{(index $.StatefulSets 0).Spec.ServiceName}}-{{$i}} $MYSQL_PRIMARY_{{$i}}:3306 & {{end}}

3. To connect to database

  Assume WeSQL leader node is {{(index .Services 0).Name}}
  In practice, we can get cluster node role by sql " select * from information_schema.wesql_cluster_local; ".
  
  MYSQL_ROOT_PASSWORD=$(kubectl get secret --namespace {{.Cluster.Namespace}} {{.Cluster.Name}} -o jsonpath="{.data.rootPassword}" | base64 -d)
  mysql -h {{.HostIP}} -uroot -p"$MYSQL_ROOT_PASSWORD" -P$MYSQL_PRIMARY_0
  
4. To bench the database
	
  # run tpcc benchmark 1 minute
  dbctl bench --host {{.HostIP}} --port $MYSQL_PRIMARY_0 --password "$MYSQL_ROOT_PASSWORD" tpcc prepare|run|clean  

5. To view the Grafana:

  open http://{{.HostIP}}:{{.GrafanaPort}}/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql_for_demo
  User: {{.GrafanaUser}}
  Password: {{.GrafanaPasswd}}

6. uninstall Playground: 

  dbctl playground destroy

--------------------------------------------------------------------
To view this guide: dbctl playground guide
To get more help: dbctl help
{{if ne .CloudProvider "local"}}To login to remote host:              ssh -i ~/.opendbaas/ssh/id_rsa ec2-user@{{.HostIP}}{{end}}
Use "dbctl [command] --help" for more information about a command.

`
