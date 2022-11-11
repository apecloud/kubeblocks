/*
Copyright ApeCloud Inc.

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

package appversion

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	EnvExperimentalExpose = "DBCTL_EXPERIMENTAL_EXPOSE"
)

func init() {
	_ = viper.BindEnv(EnvExperimentalExpose)
}

// NewAppVersionCmd creates the appversion command
func NewAppVersionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appversion",
		Short: "Database appversion operation command",
	}

	// add subcommands
	cmd.AddCommand(
		NewListCmd(f, streams),
		NewDeleteCmd(f, streams),
	)

	return cmd
}
