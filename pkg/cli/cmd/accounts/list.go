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

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/spf13/cobra"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
)

type ListUserOptions struct {
	*AccountBaseOptions
}

func NewListUserOptions(f cmdutil.Factory, streams genericiooptions.IOStreams) *ListUserOptions {
	return &ListUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams),
	}
}
func (o ListUserOptions) Validate(args []string) error {
	return o.AccountBaseOptions.Validate(args)
}

func (o *ListUserOptions) Complete(f cmdutil.Factory) error {
	return o.AccountBaseOptions.Complete(f)
}

func (o *ListUserOptions) Run(cmd *cobra.Command, f cmdutil.Factory, streams genericiooptions.IOStreams) error {
	klog.V(1).Info(fmt.Sprintf("connect to cluster %s, component %s, instance %s\n", o.ClusterName, o.ComponentName, o.PodName))
	lorryClient, err := client.NewK8sExecClientWithPod(o.Pod)
	if err != nil {
		return err
	}

	users, err := lorryClient.ListUsers(context.Background())
	if err != nil {
		o.printGeneralInfo("fail", err.Error())
		return err
	}
	o.printUserInfo(users)
	return nil
}
