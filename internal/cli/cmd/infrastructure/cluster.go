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
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	kubekeyapiv1alpha2 "github.com/kubesphere/kubekey/v3/cmd/kk/apis/kubekey/v1alpha2"
	"github.com/kubesphere/kubekey/v3/cmd/kk/pkg/common"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/tasks"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/infrastructure/types"
	cfgcore "github.com/apecloud/kubeblocks/internal/configuration"
)

type clusterOptions struct {
	IOStreams genericclioptions.IOStreams

	clusterName string
	nodes       []string
	cluster     types.Cluster

	timeout        int64
	userName       string
	password       string
	privateKey     string
	privateKeyPath string
}

func buildCommonFlags(cmd *cobra.Command, o *clusterOptions) {
	cmd.Flags().StringVarP(&o.clusterName, "name", "", "", "Specify kubernetes cluster name")
	cmd.Flags().StringSliceVarP(&o.nodes, "nodes", "", nil, "List of machines on which kubernetes is installed. [require]")

	// for user
	cmd.Flags().StringVarP(&o.userName, "user", "u", "", "Specify the account to access the remote server. [require]")
	cmd.Flags().Int64VarP(&o.timeout, "timeout", "t", 30, "Specify the ssh timeout.[option]")
	cmd.Flags().StringVarP(&o.password, "password", "p", "", "Specify the password for the account to execute sudo. [option]")
	cmd.Flags().StringVarP(&o.password, "sandbox-image", "", tasks.DefaultSandBoxImage, "Specified image will not be used by the cri. [option]")
	cmd.Flags().StringVarP(&o.privateKey, "private-key", "", "", "The PrimaryKey for ssh to the remote machine. [option]")
	cmd.Flags().StringVarP(&o.privateKeyPath, "private-key-path", "", "", "Specify the file PrimaryKeyPath of ssh to the remote machine. default ~/.ssh/id_rsa.")

	cmd.Flags().StringSliceVarP(&o.cluster.ETCD, "etcd", "", nil, "Specify etcd nodes")
	cmd.Flags().StringSliceVarP(&o.cluster.Master, "master", "", nil, "Specify master nodes")
	cmd.Flags().StringSliceVarP(&o.cluster.Worker, "worker", "", nil, "Specify worker nodes")
}

func (o *clusterOptions) Complete() error {
	if o.clusterName == "" {
		o.clusterName = "kubeblocks-" + rand.String(6)
		fmt.Printf("The cluster name is not set, auto generate cluster name: %s\n", o.clusterName)
	}

	if o.userName == "" {
		currentUser, err := user.Current()
		if err != nil {
			return err
		}
		o.userName = currentUser.Username
		fmt.Printf("The user is not set, use current user %s\n", o.userName)
	}
	if o.privateKey == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if o.privateKeyPath == "" && o.password == "" {
			o.privateKeyPath = filepath.Join(home, ".ssh", "id_rsa")
		}
		if strings.HasPrefix(o.privateKeyPath, "~/") {
			o.privateKeyPath = filepath.Join(home, o.privateKeyPath[2:])
		}
	}
	if len(o.nodes) == 0 {
		return cfgcore.MakeError("The list of machines where kubernetes is installed must be specified.")
	}
	o.cluster.Nodes = make([]types.ClusterNode, len(o.nodes))
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
		o.cluster.Nodes[i] = n
	}
	return nil
}

func (o *clusterOptions) Validate() error {
	checkFn := func(n string) bool {
		for _, node := range o.cluster.Nodes {
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
	if o.userName == "" {
		return cfgcore.MakeError("user name is empty")
	}
	if o.privateKey == "" && o.privateKeyPath != "" {
		if _, err := os.Stat(o.privateKeyPath); err != nil {
			return err
		}
		b, err := os.ReadFile(o.privateKeyPath)
		if err != nil {
			return err
		}
		o.privateKey = string(b)
	}
	if len(o.cluster.ETCD) == 0 || len(o.cluster.Master) == 0 || len(o.cluster.Worker) == 0 {
		return cfgcore.MakeError("etcd, master or worker is empty")
	}
	if err := validateNodes(o.cluster.ETCD); err != nil {
		return err
	}
	if err := validateNodes(o.cluster.Master); err != nil {
		return err
	}
	if err := validateNodes(o.cluster.Worker); err != nil {
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
