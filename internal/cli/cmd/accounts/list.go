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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type ListUserOptions struct {
	*AccountBaseOptions
}

func NewListUserOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *ListUserOptions {
	return &ListUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, sqlchannel.ListUsersOp),
	}
}
func (o ListUserOptions) Validate(args []string) error {
	return o.AccountBaseOptions.Validate(args)
}

func (o *ListUserOptions) Complete(f cmdutil.Factory) error {
	return o.AccountBaseOptions.Complete(f)
}
