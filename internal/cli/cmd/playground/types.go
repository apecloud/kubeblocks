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
)

const (
	defaultCloudProvider = cloudprovider.Local
	defaultClusterDef    = "apecloud-mysql"

	// defaultNamespace is the namespace of playground cluster
	defaultNamespace = "default"
)

var (
	// kbClusterName is the playground cluster name that created by KubeBlocks
	kbClusterName = "mycluster"
	// k8sClusterName is the k3d cluster name for playground
	k8sClusterName = "kb-playground"
)

var guideStr = `
1. Basic commands for cluster:

  kbcli cluster list                     # list database cluster and check its status
  kbcli cluster describe %[1]s       # get cluster information

2. Connect to database

  kbcli cluster connect %[1]s
  
3. View the Grafana:

  kbcli dashboard open kubeblocks-grafana
	
4. Destroy Playground:

  kbcli playground destroy

--------------------------------------------------------------------
To get more help: kbcli help
Use "kbcli [command] --help" for more information about a command.
`
