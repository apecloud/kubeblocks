/*
Copyright ApeCloud, Inc.

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
	"github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/version"
)

const (
	defaultCloudProvider = cloudprovider.Local
	defaultClusterDef    = "apecloud-mysql"

	localHost = "127.0.0.1"

	// defaultNamespace is the namespace of playground cluster
	defaultNamespace = "default"

	// CliDockerNetwork is docker network for k3d cluster when `kbcli playground`
	// all cluster will be created in this network, so they can communicate with each other
	CliDockerNetwork = "k3d-kbcli-playground"
)

var (
	// kbClusterName is the playground cluster name that created by KubeBlocks
	kbClusterName = "mycluster"
	// k8sClusterName is the k3d cluster name for playground
	k8sClusterName = "kb-playground"

	// K3sImage is k3s image repo
	K3sImage = "rancher/k3s:" + version.K3sImageTag
	// K3dToolsImage is k3d tools image repo
	K3dToolsImage = "docker.io/apecloud/k3d-tools:" + version.K3dVersion
	// K3dProxyImage is k3d proxy image repo
	K3dProxyImage = "docker.io/apecloud/k3d-proxy:" + version.K3dVersion
)

type clusterInfo struct {
	Name          string
	HostIP        string
	KubeConfig    string
	CloudProvider string
}

var guideTmpl = `
1. Basic commands for cluster:

  export KUBECONFIG={{.KubeConfig}}

  kbcli cluster list                     # list database cluster and check its status
  kbcli cluster describe {{.Name}}       # get cluster information

2. Connect to database

  kbcli cluster connect {{.Name}}
  
3. View the Grafana:

  kbcli dashboard open kubeblocks-grafana
	
4. Uninstall Playground:

  kbcli playground destroy

--------------------------------------------------------------------
To view this guide: kbcli playground guide
To get more help: kbcli help
{{if ne .CloudProvider "local"}}To login to remote host:              ssh -i ~/.kubeblocks/ssh/id_rsa ec2-user@{{.HostIP}}{{end}}
Use "kbcli [command] --help" for more information about a command.

`
