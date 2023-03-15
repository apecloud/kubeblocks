package accounts

import (
	"github.com/sethvargo/go-password/password"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type CreateUserOptions struct {
	*AccountBaseOptions
	info sqlchannel.UserInfo
}

func NewCreateUserOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *CreateUserOptions {
	return &CreateUserOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, sqlchannel.CreateUserOp),
	}
}

func (o *CreateUserOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.info.UserName, "username", "u", "", "Required. Specify the name of user, which must be unique.")
	cmd.Flags().StringVarP(&o.info.Password, "password", "p", "", "Optional. Specify the password of user. The default value is empty, which means a random password will be generated.")
	// TODO:@shanshan add expire flag if needed
	// cmd.Flags().DurationVar(&o.info.ExpireAt, "expire", 0, "Optional. Specify the expire time of password. The default value is 0, which means the user will never expire.")
}

func (o CreateUserOptions) Validate(args []string) error {
	if err := o.AccountBaseOptions.Validate(args); err != nil {
		return err
	}
	if len(o.info.UserName) == 0 {
		return errMissingUserName
	}
	return nil
}

func (o *CreateUserOptions) Complete(f cmdutil.Factory) error {
	var err error
	if err = o.AccountBaseOptions.Complete(f); err != nil {
		return err
	}
	// complete other options
	if len(o.info.Password) == 0 {
		o.info.Password, _ = password.Generate(10, 2, 0, false, false)
	}
	// encode user info to metatdata
	o.RequestMeta, err = struct2Map(o.info)
	return err
}
