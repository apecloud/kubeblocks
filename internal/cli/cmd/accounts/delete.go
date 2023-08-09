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
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
	channelutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

type DeleteUserOptions struct {
	*AccountBaseOptions
	AutoApprove bool
	info        channelutil.UserInfo
}

func NewDeleteUserOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *DeleteUserOptions {
	return &DeleteUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, channelutil.DeleteUserOp),
	}
}

func (o *DeleteUserOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVar(&o.info.UserName, "name", "", "Required user name, please specify it.")
	_ = cmd.MarkFlagRequired("name")
}

func (o *DeleteUserOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.info.UserName) == 0 {
		return errMissingUserName
	}
	if o.AutoApprove {
		return nil
	}
	if err := prompt.Confirm([]string{o.info.UserName}, o.In, ""); err != nil {
		return err
	}
	return nil
}

func (o *DeleteUserOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	o.RequestMeta, err = struct2Map(o.info)
	return err
}
