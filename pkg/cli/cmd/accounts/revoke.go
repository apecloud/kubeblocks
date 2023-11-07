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
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/pkg/lorry/client"
	lorryutil "github.com/apecloud/kubeblocks/pkg/lorry/util"
)

type RevokeOptions struct {
	*AccountBaseOptions
	userName string
	roleName string
}

func NewRevokeOptions(f cmdutil.Factory, streams genericiooptions.IOStreams) *RevokeOptions {
	return &RevokeOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams),
	}
}

func (o *RevokeOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.userName, "name", "", "Required user name, please specify it.")
	cmd.Flags().StringVarP(&o.roleName, "role", "r", "", "Role name should be one of [SUPERUSER, READWRITE, READONLY].")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("role")
}

func (o *RevokeOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.userName) == 0 {
		return errMissingUserName
	}
	if len(o.roleName) == 0 {
		return errMissingRoleName
	}
	if err := o.validRoleName(); err != nil {
		return err
	}
	return nil
}

func (o *RevokeOptions) validRoleName() error {
	candidates := []string{string(lorryutil.SuperUserRole), string(lorryutil.ReadWriteRole), string(lorryutil.ReadOnlyRole)}
	if slices.Contains(candidates, strings.ToLower(o.roleName)) {
		return nil
	}
	return errInvalidRoleName
}

func (o *RevokeOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	return err
}

func (o *RevokeOptions) Run(cmd *cobra.Command, f cmdutil.Factory, streams genericiooptions.IOStreams) error {
	klog.V(1).Info(fmt.Sprintf("connect to cluster %s, component %s, instance %s\n", o.ClusterName, o.ComponentName, o.PodName))
	lorryClient, err := client.NewK8sExecClientWithPod(o.Pod)
	if err != nil {
		return err
	}

	err = lorryClient.RevokeUserRole(context.Background(), o.userName, o.roleName)
	if err != nil {
		o.printGeneralInfo("fail", err.Error())
		return err
	}
	o.printGeneralInfo("success", "")
	return nil
}
