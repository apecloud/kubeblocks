/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	candidates := []string{sqlchannel.SuperUserRole, sqlchannel.ReadWriteRole, sqlchannel.ReadOnlyRole}
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
