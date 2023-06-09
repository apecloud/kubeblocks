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
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/builder"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type clusterOptions struct {
	types.Cluster
	IOStreams genericclioptions.IOStreams

	clusterConfig string
	clusterName   string
	nodes         []string
	timeout       int64
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

	cmd.Flags().StringSliceVarP(&o.ETCD, "etcd", "", nil, "Specify etcd nodes")
	cmd.Flags().StringSliceVarP(&o.Master, "master", "", nil, "Specify master nodes")
	cmd.Flags().StringSliceVarP(&o.Worker, "worker", "", nil, "Specify worker nodes")
}

func (o *clusterOptions) Complete() error {
	if o.clusterName == "" {
		o.clusterName = "kubeblocks-" + rand.String(6)
		fmt.Printf("The cluster name is not set, auto generate cluster name: %s\n", o.clusterName)
	}

	if o.clusterConfig != "" {
		return o.validateClusterConfig(o.clusterConfig)
	}

	if o.User.Name == "" {
		currentUser, err := user.Current()
		if err != nil {
			return err
		}
		o.User.Name = currentUser.Username
		fmt.Printf("The user is not set, use current user %s\n", o.User.Name)
	}
	if o.User.PrivateKey == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if o.User.PrivateKeyPath == "" && o.User.Password == "" {
			o.User.PrivateKeyPath = filepath.Join(home, ".ssh", "id_rsa")
		}
		if strings.HasPrefix(o.User.PrivateKeyPath, "~/") {
			o.User.PrivateKeyPath = filepath.Join(home, o.User.PrivateKeyPath[2:])
		}
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
	checkFn := func(n string) bool {
		for _, node := range o.Nodes {
			if node.Name == n {
				return true
			}
		}
		return false
	}
	validateNodes := func(nodes []string) error {
		sets := set.NewLinkedHashSetString()
		for _, node := range nodes {
			if !checkFn(node) {
				return cfgcore.MakeError("node %s is not exist!", node)
			}
			if sets.InArray(node) {
				return cfgcore.MakeError("node %s is repeat!", node)
			}
			sets.Add(node)
		}
		return nil
	}
	if o.User.Name == "" {
		return cfgcore.MakeError("user name is empty")
	}
	if o.User.PrivateKey == "" && o.User.PrivateKeyPath != "" {
		if _, err := os.Stat(o.User.PrivateKeyPath); err != nil {
			return err
		}
		b, err := os.ReadFile(o.User.PrivateKeyPath)
		if err != nil {
			return err
		}
		o.User.PrivateKey = string(b)
	}
	if len(o.ETCD) == 0 || len(o.Master) == 0 || len(o.Worker) == 0 {
		return cfgcore.MakeError("etcd, master or worker is empty")
	}
	if err := validateNodes(o.ETCD); err != nil {
		return err
	}
	if err := validateNodes(o.Master); err != nil {
		return err
	}
	if err := validateNodes(o.Worker); err != nil {
		return err
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

func (o *clusterOptions) validateClusterConfig(configFile string) error {
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
	return nil
}
