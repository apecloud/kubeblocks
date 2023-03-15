package accounts

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/delete"
	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type DeleteUserOptions struct {
	*AccountBaseOptions
	info sqlchannel.UserInfo
}

func NewDeleteUserOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *DeleteUserOptions {
	return &DeleteUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, sqlchannel.DeleteUserOp),
	}
}

func (o *DeleteUserOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.info.UserName, "username", "u", "", "Required. Specify the name of user")
}

func (o DeleteUserOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.info.UserName) == 0 {
		return errMissingUserName
	}
	if err := delete.Confirm([]string{o.info.UserName}, o.In); err != nil {
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
