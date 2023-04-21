/*
Copyright (C) 2022 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package playground

import (
	"github.com/pkg/errors"

	"github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

const (
	yesStr = "yes"
)

const (
	defaultCloudProvider = cloudprovider.Local
	defaultClusterDef    = "apecloud-mysql"

	// defaultNamespace is the namespace of playground cluster
	defaultNamespace = "default"

	// stateFileName is the file name of playground state file
	stateFileName = "kb-playground.state"

	// CloudClusterNamePrefix the prefix of cloud kubernetes cluster name
	cloudClusterNamePrefix = "kb-playground"
)

var (
	// kbClusterName is the playground cluster name that created by KubeBlocks
	kbClusterName = "mycluster"

	// defaultKubeConfigPath is the default kubeconfig path, it is ~/.kube/config
	defaultKubeConfigPath = util.ConfigPath("config")
)

// errors
var (
	kubeClusterUnreachableErr = errors.New("Kubernetes cluster unreachable")
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
