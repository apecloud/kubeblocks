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
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	channelutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

type GrantOptions struct {
	*AccountBaseOptions
	info channelutil.UserInfo
}

func NewGrantOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, op bindings.OperationKind) *GrantOptions {
	if (op != channelutil.GrantUserRoleOp) && (op != channelutil.RevokeUserRoleOp) {
		klog.V(1).Infof("invalid operation kind: %s", op)
		return nil
	}
	return &GrantOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, op),
	}
}

func (o *GrantOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.info.UserName, "name", "", "Required. Specify the name of user.")
	cmd.Flags().StringVarP(&o.info.RoleName, "role", "r", "", "Role name should be one of {SUPERUSER, READWRITE, READONLY}")
}

func (o *GrantOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.info.UserName) == 0 {
		return errMissingUserName
	}
	if len(o.info.RoleName) == 0 {
		return errMissingRoleName
	}
	if err := o.validRoleName(); err != nil {
		return err
	}
	return nil
}

func (o *GrantOptions) validRoleName() error {
	candidates := []string{string(channelutil.SuperUserRole), string(channelutil.ReadWriteRole), string(channelutil.ReadOnlyRole)}
	if slices.Contains(candidates, strings.ToLower(o.info.RoleName)) {
		return nil
	}
	return errInvalidRoleName
}

func (o *GrantOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	o.RequestMeta, err = struct2Map(o.info)
	return err
}
