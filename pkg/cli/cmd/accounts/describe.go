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

package accounts

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
)

type DescribeUserOptions struct {
	*AccountBaseOptions
	userName string
}

func NewDescribeUserOptions(f cmdutil.Factory, streams genericiooptions.IOStreams) *DescribeUserOptions {
	return &DescribeUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams),
	}
}

func (o *DescribeUserOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.userName, "name", "", "Required user name, please specify it.")
	_ = cmd.MarkFlagRequired("name")
}

func (o DescribeUserOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.userName) == 0 {
		return errMissingUserName
	}
	return nil
}

func (o *DescribeUserOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	return err
}

func (o *DescribeUserOptions) Run(cmd *cobra.Command, f cmdutil.Factory, streams genericiooptions.IOStreams) error {
	klog.V(1).Info(fmt.Sprintf("connect to cluster %s, component %s, instance %s\n", o.ClusterName, o.ComponentName, o.PodName))
	lorryClient, err := client.NewK8sExecClientWithPod(o.Pod)
	if err != nil {
		return err
	}

	user, err := lorryClient.DescribeUser(context.Background(), o.userName)
	if err != nil {
		o.printGeneralInfo("fail", err.Error())
		return err
	}
	o.printRoleInfo([]map[string]any{user})
	return nil
}
