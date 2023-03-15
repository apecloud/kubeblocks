package accounts

import (
	"strings"

	"github.com/dapr/components-contrib/bindings"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	klog "k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/sqlchannel"
)

type GrantOptions struct {
	*AccountBaseOptions
	info sqlchannel.UserInfo
}

func NewGrantOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, op bindings.OperationKind) *GrantOptions {
	if (op != sqlchannel.GrantUserRoleOp) && (op != sqlchannel.RevokeUserRoleOp) {
		klog.V(1).Infof("invalid operation kind: %s", op)
		return nil
	}
	return &GrantOptions{
		AccountBaseOptions: NewAccountBaseOptions(f, streams, op),
	}
}

func (o *GrantOptions) AddFlags(cmd *cobra.Command) {
	o.AccountBaseOptions.AddFlags(cmd)
	cmd.Flags().StringVarP(&o.info.UserName, "username", "u", "", "Required. Specify the name of user.")
	cmd.Flags().StringVarP(&o.info.RoleName, "role", "r", "", "Role name should be one of {SUPERUSER, READWRITE, READONLY}")
}

func (o GrantOptions) Validate(args []string) error {
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
	candiates := []string{sqlchannel.SuperUserRole, sqlchannel.ReadWriteRole, sqlchannel.ReadOnlyRole}
	if slices.Contains(candiates, strings.ToLower(o.info.RoleName)) {
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
