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

package cluster

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/accounts"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

var (
	createUserExamples = templates.Examples(`
		# create account
		kbcli cluster create-account NAME --component COMPNAME --username NAME --password PASSWD
		# create account without password
		kbcli cluster create-account NAME --component COMPNAME --username NAME
		# create account with expired interval
		kbcli cluster create-account NAME --component COMPNAME --username NAME --password PASSWD --expiredAt 2046-01-02T15:04:05Z
 `)

	deleteUserExamples = templates.Examples(`
		# delete account by name
		kbcli cluster delete-account NAME --component COMPNAME --username NAME
 `)

	descUserExamples = templates.Examples(`
		# describe account and show role information
		kbcli cluster describe-account NAME --component COMPNAME--username NAME
 `)

	listUsersExample = templates.Examples(`
		# list all users from specified component of a cluster
		kbcli cluster list-accounts NAME --component COMPNAME --show-connected-users

		# list all users from cluster's one particular instance
		kbcli cluster list-accounts NAME -i INSTANCE
	`)
	grantRoleExamples = templates.Examples(`
		# grant role to user
		kbcli cluster grant-role NAME --component COMPNAME --username NAME --role ROLENAME
	`)
	revokeRoleExamples = templates.Examples(`
		# revoke role from user
		kbcli cluster revoke-role NAME --component COMPNAME --role ROLENAME
	`)
)

func NewCreateAccountCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewCreateUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "create-account",
		Short:             "Create account for a cluster",
		Example:           createUserExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewDeleteAccountCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewDeleteUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "delete-account",
		Short:             "Delete account for a cluster",
		Example:           deleteUserExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewDescAccountCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewDescribeUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "describe-account",
		Short:             "Describe account roles and related information",
		Example:           descUserExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewListAccountsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewListUserOptions(f, streams)

	cmd := &cobra.Command{
		Use:               "list-accounts",
		Short:             "List accounts for a cluster",
		Aliases:           []string{"ls-accounts"},
		Example:           listUsersExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewGrantOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewGrantOptions(f, streams, sqlchannel.GrantUserRoleOp)

	cmd := &cobra.Command{
		Use:               "grant-role",
		Short:             "Grant role to account",
		Aliases:           []string{"grant", "gr"},
		Example:           grantRoleExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func NewRevokeOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewGrantOptions(f, streams, sqlchannel.RevokeUserRoleOp)

	cmd := &cobra.Command{
		Use:               "revoke-role",
		Short:             "Revoke role from account",
		Aliases:           []string{"revoke", "rv"},
		Example:           revokeRoleExamples,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.ClusterGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate(args))
			cmdutil.CheckErr(o.Complete(f))
			cmdutil.CheckErr(o.Run(f, streams))
		},
	}
	o.AddFlags(cmd)
	return cmd
}
