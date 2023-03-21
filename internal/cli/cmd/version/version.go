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

package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/version"
)

type versionOptions struct {
	verbose bool
}

// NewVersionCmd the version command
func NewVersionCmd(f cmdutil.Factory) *cobra.Command {
	o := &versionOptions{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information, include kubernetes, KubeBlocks and kbcli version.",
		Run: func(cmd *cobra.Command, args []string) {
			o.Run(f)
		},
	}
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "print detailed kbcli information")
	return cmd
}

func (o *versionOptions) Run(f cmdutil.Factory) {
	client, err := f.KubernetesClientSet()
	if err != nil {
		klog.V(1).Infof("failed to get clientset: %v", err)
	}

	versionInfo, _ := util.GetVersionInfo(client)
	if v := versionInfo[util.KubernetesApp]; len(v) > 0 {
		fmt.Printf("Kubernetes: %s\n", v)
	}
	if v := versionInfo[util.KubeBlocksApp]; len(v) > 0 {
		fmt.Printf("KubeBlocks: %s\n", v)
	}
	fmt.Printf("kbcli: %s\n", versionInfo[util.KBCLIApp])
	if o.verbose {
		fmt.Printf("  BuildDate: %s\n", version.BuildDate)
		fmt.Printf("  GitCommit: %s\n", version.GitCommit)
		fmt.Printf("  GitTag: %s\n", version.GitVersion)
		fmt.Printf("  GoVersion: %s\n", runtime.Version())
		fmt.Printf("  Compiler: %s\n", runtime.Compiler)
		fmt.Printf("  Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	}
}
