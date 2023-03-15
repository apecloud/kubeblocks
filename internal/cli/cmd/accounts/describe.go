package accounts

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	sqlchannel "github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type DescribeUserOptions struct {
	*AccountBaseOptions
	info sqlchannel.UserInfo
}

func NewDescribeUserOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *DescribeUserOptions {
	return &DescribeUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, sqlchannel.DescribeUserOp),
	}
}

func (o *DescribeUserOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.info.UserName, "username", "u", "", "Required. Specify the name of user")
}

func (o DescribeUserOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.info.UserName) == 0 {
		return errMissingUserName
	}
	return nil
}

func (o *DescribeUserOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	o.RequestMeta, err = struct2Map(o.info)
	return err
}
