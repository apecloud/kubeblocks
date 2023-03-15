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
		# create user
		kbcli cluster create-user NAME --component-name COMPNAME --username NAME --password PASSWD
		# create user without password
		kbcli cluster create-user NAME --component-name COMPNAME --username NAME
		# create user with expired interval
		kbcli cluster create-user NAME --component-name COMPNAME --username NAME --password PASSWD --expiredAt 2046-01-02T15:04:05Z
 `)

	deleteUserExamples = templates.Examples(`
		# delete user by name
		kbcli cluster delete-user NAME --component-name COMPNAME --username NAME
 `)

	descUserExamples = templates.Examples(`
		# describe user and show role information
		kbcli cluster desc-user NAME --component-name COMPNAME--username NAME
 `)

	listUsersExample = templates.Examples(`
		# list all users from specified component of a cluster
		kbcli cluster list-users NAME --component-name COMPNAME --show-connected-users

		# list all users of a cluster, by default the first component will be used
		kbcli cluster list-users NAME --show-connected-users

		# list all users from cluster's one particular instance
		kbcli cluster list-users NAME -i INSTANCE
	`)
	grantRoleExamples = templates.Examples(`
		# grant role to user
		kbcli cluster grant-role NAME --component-name COMPNAME --username NAME --role ROLENAME
	`)
	revokeRoleExamples = templates.Examples(`
		# revoke role from user
		kbcli cluster revoke-role NAME --component-name COMPNAME --role ROLENAME
	`)
)

func NewCreateUserCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewCreateUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "create-user",
		Short:             "Create user for a cluster",
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

func NewDeleteUserCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewDeleteUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "delete-user",
		Short:             "Delete user for a cluster",
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

func NewDescUserCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewDescribeUserOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "desc-user",
		Short:             "Describe user roles and related information",
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

func NewListUsersCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := accounts.NewListUserOptions(f, streams)

	cmd := &cobra.Command{
		Use:               "list-users",
		Short:             "List users for a cluster",
		Aliases:           []string{"ls-users"},
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
		Short:             "Grant role to user",
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
		Short:             "Revoke role from user",
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
