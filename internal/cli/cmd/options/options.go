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

package options

import (
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

// NewCmdOptions implements the options command
func NewCmdOptions(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Print the list of flags inherited by all commands.",
		Example: `
	# Print flags inherited by all commands
	kbcli options`,
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Usage()
		},
	}

	cmd.SetOut(out)
	cmd.SetErr(out)

	templates.UseOptionsTemplates(cmd)
	return cmd
}
