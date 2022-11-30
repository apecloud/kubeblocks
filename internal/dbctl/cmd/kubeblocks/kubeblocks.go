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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
	"github.com/apecloud/kubeblocks/internal/dbctl/util"
	"github.com/apecloud/kubeblocks/internal/dbctl/util/helm"
	"github.com/apecloud/kubeblocks/version"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

const (
	kMonitorParam = "prometheus.enabled=true,grafana.enabled=true,dashboards.enabled=true"
)

type Options struct {
	genericclioptions.IOStreams

	HelmCfg   *action.Configuration
	Namespace string
	client    dynamic.Interface
}

type InstallOptions struct {
	Options
	Version string
	Sets    []string
	Monitor bool
	Quiet   bool
}

type addEngineOptions struct {
	options             Options
	AppVersionsByte     []byte
	ClusterDefsByte     []byte
	AppVersionsFilePath string
	ClusterDefsFilePath string
}

var (
	installExample = templates.Examples(`
	# Install KubeBlocks
	dbctl kubeblocks install
	
	# Install KubeBlocks with specified version
	dbctl kubeblocks install --version=0.2.0

	# Install KubeBlocks and enable the monitor including prometheus, grafana
	dbctl kubeblocks install --monitor=true

	# Install KubeBlocks with other settings, for example, set replicaCount to 3
	dbctl kubeblocks install --set replicaCount=3
`)

	uninstallExample = templates.Examples(`
		# uninstall KubeBlocks
        dbctl kubeblocks uninstall`)
)

// NewKubeBlocksCmd creates the kubeblocks command
func NewKubeBlocksCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kubeblocks [install | uninstall | add-engine]",
		Short: "KubeBlocks operation commands",
	}
	cmd.AddCommand(
		newInstallCmd(f, streams),
		newUninstallCmd(f, streams),
		newAddEngineCmd(f, streams),
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

	if o.HelmCfg, err = helm.NewActionConfig(o.Namespace, kubeconfig); err != nil {
		return err
	}

	o.client, err = f.DynamicClient()
	return err
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
	notes, err := o.installChart()
	if err != nil {
		return err
	}

	// print notes
	if !o.Quiet {
		o.printNotes(notes)
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
		Name:      types.KubeBlocksChartName,
		Chart:     types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:      true,
		Version:   o.Version,
		Namespace: o.Namespace,
		Sets:      sets,
		Login:     true,
		TryTimes:  2,
	}
	notes, err := chart.Install(o.HelmCfg)
	if err != nil {
		return "", err
	}
	return notes, nil
}

func (o *InstallOptions) printNotes(notes string) {
	fmt.Fprintf(o.Out, `
KubeBlocks %s Install SUCCESSFULLY!

-> Basic commands for cluster:
    dbctl cluster create -h     # help information about creating a database cluster
    dbctl cluster list          # list all database clusters
    dbctl cluster describe <cluster name>  # get cluster information

-> Uninstall DBaaS:
    dbctl kubeblocks uninstall
`, o.Version)
	fmt.Fprint(o.Out, notes)
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
	if err := removeFinalizers(o.client); err != nil {
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

	// patch appversion's finalizer
	appVerList, err := client.Resource(types.AppVersionGVR()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, appVer := range appVerList.Items {
		if _, err = client.Resource(types.AppVersionGVR()).Patch(ctx, appVer.GetName(), k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"remove\", \"path\": \"/metadata/finalizers\"}]"), metav1.PatchOptions{}); err != nil {
			return err
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
			util.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", false, "Set monitor enabled (default false)")
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
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func newAddEngineCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addEngineOptions{
		options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:   "add-engine",
		Short: "Add a new engine to KubeBlocks",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.AppVersionsFilePath, "app-version", "", "KubeBlocks new engine app version yaml file path")
	cmd.Flags().StringVar(&o.ClusterDefsFilePath, "cluster-definition", "", "KubeBlocks new engine cluster definition yaml file path")

	return cmd
}

func (o *addEngineOptions) Validate() error {
	if o.AppVersionsFilePath == "" && o.ClusterDefsFilePath == "" {
		return fmt.Errorf("a valid appversion yaml file or clusterdefinition yaml file path is needed")
	}
	return nil
}

func (o *addEngineOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var (
		appVersionsByte []byte
		clusterDefsByte []byte
		err             error
	)
	if len(o.AppVersionsFilePath) > 0 {
		if appVersionsByte, err = os.ReadFile(o.AppVersionsFilePath); err != nil {
			return err
		}
		if appVersionsByte, err = yaml.YAMLToJSON(appVersionsByte); err != nil {
			return err
		}
		o.AppVersionsByte = appVersionsByte
	}
	if len(o.ClusterDefsFilePath) > 0 {
		if clusterDefsByte, err = os.ReadFile(o.ClusterDefsFilePath); err != nil {
			return err
		}
		if clusterDefsByte, err = yaml.YAMLToJSON(clusterDefsByte); err != nil {
			return err
		}
		o.ClusterDefsByte = clusterDefsByte
	}
	err = o.options.complete(f, cmd)
	if err != nil {
		return err
	}
	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *addEngineOptions) Run() error {
	var (
		err             error
		unstructuredObj *unstructured.Unstructured
	)
	if o.ClusterDefsFilePath != "" {
		if err = json.Unmarshal(o.ClusterDefsByte, &unstructuredObj); err != nil {
			return err
		}
		if unstructuredObj, err = o.options.client.Resource(types.ClusterDefGVR()).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(o.options.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}
	if o.AppVersionsFilePath != "" {
		if err = json.Unmarshal(o.AppVersionsByte, &unstructuredObj); err != nil {
			return err
		}
		if unstructuredObj, err = o.options.client.Resource(types.AppVersionGVR()).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
			return err
		}
		fmt.Fprintf(o.options.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	}
	return nil
}
