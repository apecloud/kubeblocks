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

package playground

import (
	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/version"
)

const (
	defaultEngine        = "wesql"
	defaultCloudProvider = "local"
	localHost            = "127.0.0.1"
	defaultReplicas      = 3

	// CliDockerNetwork is docker network for k3d cluster when `kbcli playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-kbcli-playground"
)

var (
	// dbClusterName is the playground database cluster name
	dbClusterName = "mycluster"
	// dbClusterNamespace is the namespace of playground database cluster
	dbClusterNamespace = "default"
	// clusterName is the k3d cluster name for playground
	clusterName = "kubeblocks-playground"

	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag
	// K3dToolsImage is k3d tools image repo
	K3dToolsImage = "docker.io/apecloud/k3d-tools:" + version.K3dVersion
	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/apecloud/k3d-proxy:" + version.K3dVersion
)

type clusterInfo struct {
	*cluster.ClusterObjects

	HostIP        string
	KubeConfig    string
	CloudProvider string
}

var kubeConfig = `
apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://${KUBERNETES_API_SERVER_ADDRESS}:6444
  name: k3d-kubeblocks-playground
contexts:
- context:
    cluster: k3d-kubeblocks-playground
    user: admin@k3d-kubeblocks-playground
  name: k3d-kubeblocks-playground
current-context: k3d-kubeblocks-playground
kind: Config
preferences: {}
users:
- name: admin@k3d-kubeblocks-playground
  user:
    client-certificate-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJrRENDQVRlZ0F3SUJBZ0lJR1dEc0wyWmFtWjB3Q2dZSUtvWkl6ajBFQXdJd0l6RWhNQjhHQTFVRUF3d1kKYXpOekxXTnNhV1Z1ZEMxallVQXhOalU1TlRnek1EUTBNQjRYRFRJeU1EZ3dOREF6TVRjeU5Gb1hEVEl6TURndwpOREF6TVRjeU5Gb3dNREVYTUJVR0ExVUVDaE1PYzNsemRHVnRPbTFoYzNSbGNuTXhGVEFUQmdOVkJBTVRESE41CmMzUmxiVHBoWkcxcGJqQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VIQTBJQUJKbkxHR1FNUmZva2srWDcKSS9HNWRSbG5sUzYwODlqWGV3Q0l1OGVvNmc5bUVlU203NWRmdzc2R2IrZ29BbXFXK244MkNqRVd1QTNrSEQyeQpQTUxSS2JhalNEQkdNQTRHQTFVZER3RUIvd1FFQXdJRm9EQVRCZ05WSFNVRUREQUtCZ2dyQmdFRkJRY0RBakFmCkJnTlZIU01FR0RBV2dCU1Fhd1VYVEZjMzVCdWJkQkdrK3ExZXZ4VW5SVEFLQmdncWhrak9QUVFEQWdOSEFEQkUKQWlBVXl0dWxOQzVVbnRCcmlvOGlhd1gxUUdjTEVxUENPWk04VmFETXozMTBoUUlnTWIxSHJGa3JXUHFWSTVvQgpBdyttN2szK0I5SzBWem1mcTJtSmx3V2pNdmM9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJkekNDQVIyZ0F3SUJBZ0lCQURBS0JnZ3Foa2pPUFFRREFqQWpNU0V3SHdZRFZRUUREQmhyTTNNdFkyeHAKWlc1MExXTmhRREUyTlRrMU9ETXdORFF3SGhjTk1qSXdPREEwTURNeE56STBXaGNOTXpJd09EQXhNRE14TnpJMApXakFqTVNFd0h3WURWUVFEREJock0zTXRZMnhwWlc1MExXTmhRREUyTlRrMU9ETXdORFF3V1RBVEJnY3Foa2pPClBRSUJCZ2dxaGtqT1BRTUJCd05DQUFRUWF0NDNGSFl0ZlpyT2YreHZwaFhacUEvaEFSTUhFd2JBcDBGSVdzTUcKMmlGVnZCbThBWE9MUWxYY0VKSW5EVmppZjFZYkFISWhiYVl2WjY4NXk0SzNvMEl3UURBT0JnTlZIUThCQWY4RQpCQU1DQXFRd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFRmdRVWtHc0ZGMHhYTitRYm0zUVJwUHF0ClhyOFZKMFV3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQUxZUU1qMkRqbnNRd2lKUGd0UlE3d3VDN1piMDd1VzEKZXU2SDhoaFBCN2l4QWlCbkJmQlU3M3BkSWFCdVBxNGR2TGw1MDloTWNtU1FXTVo4VVpoV1lPS0FNUT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
    client-key-data: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUM1VUgzOC91VXJVQWJZbENnSTZmU25kTEhVUi9lNFJ4L3JQNkdUMUNoeXRvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFbWNzWVpBeEYraVNUNWZzajhibDFHV2VWTHJUejJOZDdBSWk3eDZqcUQyWVI1S2J2bDEvRAp2b1p2NkNnQ2FwYjZmellLTVJhNERlUWNQYkk4d3RFcHRnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`

var guideTmpl = `
KubeBlocks playground init SUCCESSFULLY!
MySQL X-Cluster(WeSQL) "{{.Cluster.Name}}" has been CREATED!

1. Basic commands for cluster:

  export KUBECONFIG={{.KubeConfig}}

  kbcli cluster list                     # list database cluster and check its PHASE
  kbcli cluster describe {{.Cluster.Name}}       # get cluster information

2. Connect to database

  kbcli cluster connect {{.Cluster.Name}}
  
3. View the Grafana:

  export POD_NAME=$(kubectl get pods --namespace default -l "app.kubernetes.io/name=grafana,app.kubernetes.io/instance=kubeblocks" -o jsonpath="{.items[0].metadata.name}")
  kubectl --namespace default port-forward $POD_NAME 3000
  open http://{{.HostIP}}:3000/d/549c2bf8936f7767ea6ac47c47b00f2a/mysql

4. Uninstall Playground:

  kbcli playground destroy

--------------------------------------------------------------------
To view this guide: kbcli playground guide
To get more help: kbcli help
{{if ne .CloudProvider "local"}}To login to remote host:              ssh -i ~/.kubeblocks/ssh/id_rsa ec2-user@{{.HostIP}}{{end}}
Use "kbcli [command] --help" for more information about a command.

`
