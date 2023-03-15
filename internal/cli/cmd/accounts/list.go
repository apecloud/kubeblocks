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
