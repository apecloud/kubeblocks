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

package kubeblocks

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/strings/slices"

	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const (
	kMonitorParam = "prometheus.enabled=true,grafana.enabled=true,dashboards.enabled=true"
)

type Options struct {
	genericclioptions.IOStreams

	HelmCfg   *action.Configuration
	Namespace string
	dynamic   dynamic.Interface
	client    *kubernetes.Clientset
}

type InstallOptions struct {
	Options
	Version         string
	Sets            []string
	Monitor         bool
	Quiet           bool
	CreateNamespace bool
}

var (
	installExample = templates.Examples(`
	# Install KubeBlocks
	kbcli kubeblocks install
	
	# Install KubeBlocks with specified version
	kbcli kubeblocks install --version=0.2.0

	# Install KubeBlocks with other settings, for example, set replicaCount to 3
	kbcli kubeblocks install --set replicaCount=3
`)

	uninstallExample = templates.Examples(`
		# uninstall KubeBlocks
        kbcli kubeblocks uninstall`)
)

// NewKubeBlocksCmd creates the kubeblocks command
func NewKubeBlocksCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeblocks [install | upgrade | uninstall]",
		Short: "KubeBlocks operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUpgradeCmd(f, streams),
		newUninstallCmd(f, streams),
	)
	return cmd
}

func (o *Options) complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	if o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	kubeconfig, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	kubecontext, err := cmd.Flags().GetString("context")
	if err != nil {
		return err
	}

	if o.HelmCfg, err = helm.NewActionConfig(o.Namespace, kubeconfig, helm.WithContext(kubecontext)); err != nil {
		return err
	}

	if o.dynamic, err = f.DynamicClient(); err != nil {
		return err
	}

	o.client, err = f.KubernetesClientSet()
	return err
}

func (o *Options) preCheck() error {
	preCheckList := []string{
		"clusters.dbaas.kubeblocks.io",
	}
	ctx := context.Background()
	// delete crds
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  types.VersionV1,
		Resource: "customresourcedefinitions",
	}
	crdList, err := o.dynamic.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, crd := range crdList.Items {
		// find kubeblocks crds
		if strings.Contains(crd.GetName(), "kubeblocks.io") &&
			slices.Contains(preCheckList, crd.GetName()) {
			group, _, err := unstructured.NestedString(crd.Object, "spec", "group")
			if err != nil {
				return err
			}
			gvr := schema.GroupVersionResource{
				Group:    group,
				Version:  types.Version,
				Resource: strings.Split(crd.GetName(), ".")[0],
			}
			// find custom resource
			objList, err := o.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			if len(objList.Items) > 0 {
				return errors.Errorf("Can not uninstall, you should delete custom resource %s %s first", crd.GetName(), objList.Items[0].GetName())
			}
		}
	}
	return nil
}

func (o *InstallOptions) check() error {
	if o.CreateNamespace {
		return nil
	}

	// check if namespace exists
	if _, err := o.client.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{}); err != nil {
		return err
	}
	return nil
}

func (o *InstallOptions) Run() error {
	fmt.Fprintf(o.Out, "Install KubeBlocks %s\n", o.Version)

	if o.Monitor {
		o.Sets = append(o.Sets, kMonitorParam)
	}

	// Add repo, if exists, will update it
	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: types.KubeBlocksChartURL}); err != nil {
		return err
	}

	// install KubeBlocks chart
	_, err := o.installChart()
	if err != nil {
		return err
	}

	// print notes
	if !o.Quiet {
		o.printNotes()
	}

	return nil
}

func (o *InstallOptions) Upgrade() error {
	fmt.Fprintf(o.Out, "Upgrading KubeBlocks to %s\n", o.Version)

	if o.Monitor {
		o.Sets = append(o.Sets, kMonitorParam)
	}

	// Add repo, if exists, will update it
	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: types.KubeBlocksChartURL}); err != nil {
		return err
	}

	// upgrade KubeBlocks chart
	_, err := o.upgradeChart()
	if err != nil {
		return err
	}

	// print notes
	if !o.Quiet {
		o.printNotes()
	}

	return nil
}

func (o *InstallOptions) installChart() (string, error) {
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	chart := helm.InstallOpts{
		Name:            types.KubeBlocksChartName,
		Chart:           types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:            true,
		Version:         o.Version,
		Namespace:       o.Namespace,
		Sets:            sets,
		Login:           true,
		TryTimes:        2,
		CreateNamespace: o.CreateNamespace,
	}
	notes, err := chart.Install(o.HelmCfg)
	if err != nil {
		return "", err
	}
	return notes, nil
}

func (o *InstallOptions) upgradeChart() (string, error) {
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Version:   o.Version,
		Namespace: o.Namespace,
		Sets:      sets,
		Login:     true,
		TryTimes:  2,
	}
	notes, err := chart.Upgrade(o.HelmCfg)
	if err != nil {
		return "", err
	}
	return notes, nil
}

func (o *InstallOptions) printNotes() {
	fmt.Fprintf(o.Out, `
KubeBlocks %s Install SUCCESSFULLY!

-> Basic commands for cluster:
    kbcli cluster create -h     # help information about creating a database cluster
    kbcli cluster list          # list all database clusters
    kbcli cluster describe <cluster name>  # get cluster information

-> Uninstall KubeBlocks:
    kbcli kubeblocks uninstall
`, o.Version)
	if o.Monitor {
		fmt.Fprint(o.Out, `
-> To view the monitor components console(Grafana/Prometheus/AlertManager):
    kbcli dashboard list        # list all monitor components
    kbcli dashboard open <name> # open the console in the default browser
`)
	} else {
		fmt.Fprint(o.Out, `
Notes: Monitor components(Grafana/Prometheus/AlertManager) is not installed,
    use 'kbcli kubeblocks update --monitor=true' to install later.
`)
	}
}

func (o *Options) run() error {
	fmt.Fprintln(o.Out, "Uninstall KubeBlocks")

	// uninstall chart
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Namespace: o.Namespace,
	}
	if err := chart.UnInstall(o.HelmCfg); err != nil {
		return err
	}

	// remove repo
	if err := helm.RemoveRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: types.KubeBlocksChartURL}); err != nil {
		return err
	}

	// remove finalizers
	if err := removeFinalizers(o.dynamic); err != nil {
		return err
	}

	if err := deleteCRDs(o.dynamic); err != nil {
		return err
	}

	fmt.Fprintln(o.Out, "Successfully uninstall KubeBlocks")
	return nil
}

func removeFinalizers(client dynamic.Interface) error {
	// patch clusterdefinition finalizer
	ctx := context.Background()
	cdList, err := client.Resource(types.ClusterDefGVR()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, cd := range cdList.Items {
		if _, err = client.Resource(types.ClusterDefGVR()).Patch(ctx, cd.GetName(), k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
			return err
		}
	}

	// patch clusterversion's finalizer
	clusterVersionList, err := client.Resource(types.ClusterVersionGVR()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, clusterVersion := range clusterVersionList.Items {
		if _, err = client.Resource(types.ClusterVersionGVR()).Patch(ctx, clusterVersion.GetName(), k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deleteCRDs(cli dynamic.Interface) error {
	ctx := context.Background()
	// delete crds
	crdGVR := schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  types.VersionV1,
		Resource: "customresourcedefinitions",
	}
	crdList, err := cli.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, crd := range crdList.Items {
		if strings.Contains(crd.GetName(), "kubeblocks.io") {
			if err := cli.Resource(crdGVR).Delete(ctx, crd.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install KubeBlocks",
		Args:    cobra.NoArgs,
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.check())
			util.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana (default true)")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "create the namespace if not present")

	return cmd
}

func newUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrade KubeBlocks",
		Args:    cobra.NoArgs,
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.check())
			util.CheckErr(o.Upgrade())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")

	return cmd
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &Options{
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall KubeBlocks",
		Args:    cobra.NoArgs,
		Example: uninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.preCheck())
			util.CheckErr(o.run())
		},
	}
	return cmd
}
