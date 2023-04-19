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
	cmd.Flags().StringVar(&o.info.UserName, "name", "", "Required. Specify the name of user")
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
