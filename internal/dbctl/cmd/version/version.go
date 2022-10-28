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

package version

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/version"
)

type versionOptions struct {
	verbose            bool
	k8sServerVersion   string
	kubeBlocksVersions []string
	client             dynamic.Interface
	discoveryClient    discovery.CachedDiscoveryInterface
}

// NewVersionCmd the version command
func NewVersionCmd(f cmdutil.Factory) *cobra.Command {
	o := &versionOptions{}
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version information",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f))
			o.Run()
		},
	}
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "print detailed dbctl information")
	return cmd
}

func (o *versionOptions) Complete(f cmdutil.Factory) error {
	var err error
	o.kubeBlocksVersions = make([]string, 0)
	if o.client, err = f.DynamicClient(); err != nil {
		return err
	}
	o.discoveryClient, err = f.ToDiscoveryClient()
	return err
}

// initKubeBlocksVersion init KubeBlocks version
func (o *versionOptions) initKubeBlocksVersion() {
	var (
		err               error
		kubeBlocksDeploys *unstructured.UnstructuredList
	)
	// get KubeBlocks deployments in all namespaces
	gvr := schema.GroupVersionResource{Group: types.AppsGroup, Version: types.VersionV1, Resource: types.ResourceDeployments}
	if kubeBlocksDeploys, err = o.client.Resource(gvr).Namespace(metav1.NamespaceAll).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=" + types.KubeBlocksChartName,
	}); err != nil {
		return
	}
	// get KubeBlocks version
	for _, deploy := range kubeBlocksDeploys.Items {
		labels := deploy.GetLabels()
		if labels == nil {
			continue
		}
		if version, ok := labels["app.kubernetes.io/version"]; ok {
			o.kubeBlocksVersions = append(o.kubeBlocksVersions, version)
		}
	}
}

// initK8sVersion init k8s server version
func (o *versionOptions) initK8sVersion() {
	if o.discoveryClient == nil {
		return
	}
	if serverVersion, _ := o.discoveryClient.ServerVersion(); serverVersion != nil {
		o.k8sServerVersion = serverVersion.GitVersion
	}
}

func (o *versionOptions) Run() {
	o.initKubeBlocksVersion()
	o.initK8sVersion()

	if len(o.kubeBlocksVersions) > 0 {
		fmt.Printf("KubeBlocks: %s\n", strings.Join(o.kubeBlocksVersions, " "))
	}
	if len(o.k8sServerVersion) > 0 {
		fmt.Printf("Kubernetes: %s\n", o.k8sServerVersion)
	}
	fmt.Printf("Dbctl: %s\n", version.GetVersion())
	if o.verbose {
		fmt.Printf("  BuildDate: %s\n", version.BuildDate)
		fmt.Printf("  GitCommit: %s\n", version.GitCommit)
		fmt.Printf("  GitTag: %s\n", version.GitVersion)
		fmt.Printf("  GoVersion: %s\n", runtime.Version())
		fmt.Printf("  Compiler: %s\n", runtime.Compiler)
		fmt.Printf("  Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	}
}
