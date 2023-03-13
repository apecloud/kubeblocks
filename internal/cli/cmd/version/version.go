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
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/version"
)

type versionOptions struct {
	verbose bool
	client  clientset.Interface
}

// NewVersionCmd the version command
func NewVersionCmd(f cmdutil.Factory) *cobra.Command {
	o := &versionOptions{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information, include kubernetes, KubeBlocks and kbcli version.",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f))
			o.Run()
		},
	}
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "print detailed kbcli information")
	return cmd
}

func (o *versionOptions) Complete(f cmdutil.Factory) error {
	var err error
	if o.client, err = f.KubernetesClientSet(); err != nil {
		return err
	}
	return err
}

func (o *versionOptions) Run() {
	versionInfo, _ := util.GetVersionInfo(o.client)
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
