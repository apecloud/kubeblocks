/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package infrastructure

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/StudioSol/set"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/pkg/cli/cmd/infrastructure/types"
	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/util/prompt"
	cfgcore "github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
)

type clusterOptions struct {
	types.Cluster
	IOStreams genericiooptions.IOStreams

	clusterConfig string
	clusterName   string
	timeout       int64
	nodes         []string
}

func buildCommonFlags(cmd *cobra.Command, o *clusterOptions) {
	cmd.Flags().StringVarP(&o.clusterConfig, "config", "c", "", "Specify infra cluster config file. [option]")
	cmd.Flags().StringVarP(&o.clusterName, "name", "", "", "Specify kubernetes cluster name")
	cmd.Flags().StringSliceVarP(&o.nodes, "nodes", "", nil, "List of machines on which kubernetes is installed. [require]")

	// for user
	cmd.Flags().StringVarP(&o.User.Name, "user", "u", "", "Specify the account to access the remote server. [require]")
	cmd.Flags().Int64VarP(&o.timeout, "timeout", "t", 30, "Specify the ssh timeout.[option]")
	cmd.Flags().StringVarP(&o.User.Password, "password", "p", "", "Specify the password for the account to execute sudo. [option]")
	cmd.Flags().StringVarP(&o.User.PrivateKey, "private-key", "", "", "The PrimaryKey for ssh to the remote machine. [option]")
	cmd.Flags().StringVarP(&o.User.PrivateKeyPath, "private-key-path", "", "", "Specify the file PrimaryKeyPath of ssh to the remote machine. default ~/.ssh/id_rsa.")

	cmd.Flags().StringSliceVarP(&o.RoleGroup.ETCD, "etcd", "", nil, "Specify etcd nodes")
	cmd.Flags().StringSliceVarP(&o.RoleGroup.Master, "master", "", nil, "Specify master nodes")
	cmd.Flags().StringSliceVarP(&o.RoleGroup.Worker, "worker", "", nil, "Specify worker nodes")
}

func (o *clusterOptions) Complete() error {
	if o.clusterName == "" && o.clusterConfig == "" {
		o.clusterName = "kubeblocks-" + rand.String(6)
		fmt.Printf("The cluster name is not set, auto generate cluster name: %s\n", o.clusterName)
	}

	if o.clusterConfig != "" {
		return o.fillClusterConfig(o.clusterConfig)
	}

	if o.User.Name == "" {
		currentUser, err := user.Current()
		if err != nil {
			return err
		}
		o.User.Name = currentUser.Username
		fmt.Printf("The user is not set, use current user %s\n", o.User.Name)
	}
	if o.User.Password == "" && o.User.PrivateKey == "" && o.User.PrivateKeyPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		o.User.PrivateKeyPath = filepath.Join(home, ".ssh", "id_rsa")
	}
	if len(o.nodes) == 0 {
		return cfgcore.MakeError("The list of machines where kubernetes is installed must be specified.")
	}
	o.Nodes = make([]types.ClusterNode, len(o.nodes))
	for i, node := range o.nodes {
		fields := strings.SplitN(node, ":", 3)
		if len(fields) < 2 {
			return cfgcore.MakeError("The node format is incorrect, require: [name:address:internalAddress].")
		}
		n := types.ClusterNode{
			Name:            fields[0],
			Address:         fields[1],
			InternalAddress: fields[1],
		}
		if len(fields) == 3 {
			n.InternalAddress = fields[2]
		}
		o.Nodes[i] = n
	}
	return nil
}

func (o *clusterOptions) Validate() error {
	if o.User.Name == "" {
		return cfgcore.MakeError("user name is empty")
	}
	if o.clusterName == "" {
		return cfgcore.MakeError("kubernetes name is empty")
	}
	if err := validateUser(o); err != nil {
		return err
	}
	if !o.RoleGroup.IsValidate() {
		return cfgcore.MakeError("etcd, master or worker is empty")
	}
	if err := o.checkReplicaNode(o.RoleGroup.ETCD); err != nil {
		return err
	}
	if err := o.checkReplicaNode(o.RoleGroup.Master); err != nil {
		return err
	}
	if err := o.checkReplicaNode(o.RoleGroup.Worker); err != nil {
		return err
	}
	return nil
}

func (o *clusterOptions) checkReplicaNode(nodes []string) error {
	sets := cfgutil.NewSet()
	for _, node := range nodes {
		if !o.hasNode(node) {
			return cfgcore.MakeError("node %s is not exist!", node)
		}
		if sets.InArray(node) {
			return cfgcore.MakeError("node %s is repeat!", node)
		}
		sets.Add(node)
	}
	return nil
}

func (o *clusterOptions) hasNode(n string) bool {
	for _, node := range o.Nodes {
		if node.Name == n {
			return true
		}
	}
	return false
}

func (o *clusterOptions) confirm(promptStr string) (bool, error) {
	const yesStr = "yes"
	const noStr = "no"

	confirmStr := []string{yesStr, noStr}
	printer.Warning(o.IOStreams.Out, promptStr)
	input, err := prompt.NewPrompt("Please type [yes/No] to confirm:",
		func(input string) error {
			if !slices.Contains(confirmStr, strings.ToLower(input)) {
				return fmt.Errorf("typed \"%s\" does not match \"%s\"", input, confirmStr)
			}
			return nil
		}, o.IOStreams.In).Run()
	if err != nil {
		return false, err
	}
	return strings.ToLower(input) == yesStr, nil
}

func (o *clusterOptions) fillClusterConfig(configFile string) error {
	_, err := os.Stat(configFile)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}

	c, err := builder.BuildResourceFromYaml(o.Cluster, string(b))
	if err != nil {
		return err
	}
	o.Cluster = *c
	o.clusterName = c.Name
	return nil
}

func checkAndUpdateHomeDir(user *types.ClusterUser) error {
	if !strings.HasPrefix(user.PrivateKeyPath, "~/") {
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	user.PrivateKeyPath = filepath.Join(home, user.PrivateKeyPath[2:])
	return nil
}

func validateUser(o *clusterOptions) error {
	if o.User.Password != "" || o.User.PrivateKey != "" {
		return nil
	}
	if o.User.PrivateKey == "" && o.User.PrivateKeyPath != "" {
		if err := checkAndUpdateHomeDir(&o.User); err != nil {
			return err
		}
		if _, err := os.Stat(o.User.PrivateKeyPath); err != nil {
			return err
		}
		b, err := os.ReadFile(o.User.PrivateKeyPath)
		if err != nil {
			return err
		}
		o.User.PrivateKey = string(b)
	}
	return nil
}

func syncClusterNodeRole(cluster *kubekeyapiv1alpha2.ClusterSpec, runtime *common.KubeRuntime) {
	hostSet := set.NewLinkedHashSetString()
	for _, role := range cluster.GroupHosts() {
		for _, host := range role {
			if host.IsRole(common.Master) || host.IsRole(common.Worker) {
				host.SetRole(common.K8s)
			}
			if !hostSet.InArray(host.GetName()) {
				hostSet.Add(host.GetName())
				runtime.BaseRuntime.AppendHost(host)
				runtime.BaseRuntime.AppendRoleMap(host)
			}
		}
	}
}
