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

package kubeblocks

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/strings/slices"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const (
	kMonitorParam      = "prometheus.enabled=%[1]t,grafana.enabled=%[1]t,dashboards.enabled=%[1]t"
	requiredK8sVersion = "1.22.0"
)

type Options struct {
	genericclioptions.IOStreams

	HelmCfg   *action.Configuration
	Namespace string
	Client    kubernetes.Interface
	Dynamic   dynamic.Interface
}

type InstallOptions struct {
	Options
	Version         string
	Sets            []string
	Monitor         bool
	Quiet           bool
	CreateNamespace bool
	check           bool
	timeout         time.Duration
}

var (
	installExample = templates.Examples(`
	# Install KubeBlocks
	kbcli kubeblocks install
	
	# Install KubeBlocks with specified version
	kbcli kubeblocks install --version=0.4.0

	# Install KubeBlocks with other settings, for example, set replicaCount to 3
	kbcli kubeblocks install --set replicaCount=3`)

	upgradeExample = templates.Examples(`
	# Upgrade KubeBlocks to specified version
	kbcli kubeblocks upgrade --version=0.4.0

	# Upgrade KubeBlocks other settings, for example, set replicaCount to 3
	kbcli kubeblocks upgrade --set replicaCount=3`)

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

	config, err := cmd.Flags().GetString("kubeconfig")
	if err != nil {
		return err
	}

	ctx, err := cmd.Flags().GetString("context")
	if err != nil {
		return err
	}

	if o.HelmCfg, err = helm.NewActionConfig(o.Namespace, config, helm.WithContext(ctx)); err != nil {
		return err
	}

	if o.Dynamic, err = f.DynamicClient(); err != nil {
		return err
	}

	o.Client, err = f.KubernetesClientSet()
	return err
}

func (o *Options) preCheck() error {
	printer.Warning(o.Out, "uninstall will remove all KubeBlocks resources.\n")

	// wait user to confirm
	if err := confirmUninstall(o.In); err != nil {
		return err
	}

	preCheckList := []string{
		"clusters.dbaas.kubeblocks.io",
	}
	ctx := context.Background()
	// delete crds
	crs := map[string][]string{}
	crdList, err := o.Dynamic.Resource(types.CRDGVR()).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, crd := range crdList.Items {
		// find kubeblocks crds
		if strings.Contains(crd.GetName(), "kubeblocks.io") &&
			slices.Contains(preCheckList, crd.GetName()) {
			gvr, err := getGVRByCRD(&crd)
			if err != nil {
				return err
			}
			// find custom resource
			objList, err := o.Dynamic.Resource(*gvr).List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for _, item := range objList.Items {
				crs[crd.GetName()] = append(crs[crd.GetName()], item.GetName())
			}
		}
	}

	if len(crs) > 0 {
		errMsg := bytes.NewBufferString("failed to uninstall, the following custom resources need to be removed first:\n")
		for k, v := range crs {
			errMsg.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(v, " ")))
		}
		return errors.Errorf(errMsg.String())
	}

	return nil
}

func (o *InstallOptions) Install() error {
	// check if KubeBlocks has been installed
	versionInfo, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	if v := versionInfo[util.KubeBlocksApp]; len(v) > 0 {
		fmt.Fprintf(o.Out, "KubeBlocks %s already exists\n", v)
		return nil
	}

	// check if namespace exists
	if !o.CreateNamespace {
		if _, err = o.Client.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{}); err != nil {
			return err
		}
	}

	// check whether there are remained resource left by previous KubeBlocks installation, if yes,
	// output the resource name
	if err = o.checkRemainedResource(); err != nil {
		return err
	}

	if err = o.preCheck(versionInfo); err != nil {
		return err
	}

	spinner := util.Spinner(o.Out, "%-40s", "Install KubeBlocks "+o.Version)
	defer spinner(false)

	o.Sets = append(o.Sets, fmt.Sprintf(kMonitorParam, o.Monitor))

	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}

	// install KubeBlocks chart
	if err = o.installChart(); err != nil {
		return err
	}

	// successfully installed
	spinner(true)

	return nil
}

func (o *InstallOptions) preCheck(versionInfo map[util.AppName]string) error {
	if !o.check {
		return nil
	}

	versionErr := fmt.Errorf("failed to get kubernetes version")
	k8sVersionStr, ok := versionInfo[util.KubernetesApp]
	if !ok {
		return versionErr
	}

	version := util.GetK8sVersion(k8sVersionStr)
	if len(version) == 0 {
		return versionErr
	}

	// check kubernetes version
	spinner := util.Spinner(o.Out, "%-40s", "Kubernetes version "+version)
	if version >= requiredK8sVersion {
		spinner(true)
	} else {
		spinner(false)
		return fmt.Errorf("kubernetes version should be larger than or equal to %s", requiredK8sVersion)
	}

	// check kbcli version
	spinner = util.Spinner(o.Out, "%-40s", "kbcli version "+versionInfo[util.KBCLIApp])
	spinner(true)

	provider := util.GetK8sProvider(k8sVersionStr)
	if provider.IsCloud() {
		spinner = util.Spinner(o.Out, "%-40s", "Kubernetes provider "+provider)
		spinner(true)
	} else {
		// check whether user turn on features that only enable on cloud kubernetes cluster,
		// if yes, turn off these features and output message
		o.disableUnsupportedSets()
	}
	return nil
}

func (o *InstallOptions) disableUnsupportedSets() {
	var (
		newSets      []string
		disabledSets []string
	)
	// unsupported flags in non-cloud kubernetes cluster
	unsupported := []string{"loadbalancer.enabled"}

	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}

	// check all sets, remove unsupported and output message
	for _, set := range sets {
		need := true
		for _, key := range unsupported {
			if !strings.Contains(set, key) {
				continue
			}

			// found unsupported, parse its value
			kv := strings.Split(set, "=")
			if len(kv) <= 1 {
				break
			}

			// if value is false, ignore it
			val, err := strconv.ParseBool(kv[1])
			if err != nil || !val {
				break
			}

			// if value is true, remove it from original sets
			need = false
			disabledSets = append(disabledSets, key)
			break
		}
		if need {
			newSets = append(newSets, set)
		}
	}

	if len(disabledSets) == 0 {
		return
	}

	msg := "following flags are not available in current kubernetes environment, they will be disabled\n"
	if len(disabledSets) == 1 {
		msg = "following flag is not available in current kubernetes environment, it will be disabled\n"
	}
	printer.Warning(o.Out, msg)
	for _, set := range disabledSets {
		fmt.Fprintf(o.Out, "  · %s\n", set)
	}
	o.Sets = newSets
}

func (o *InstallOptions) checkRemainedResource() error {
	if !o.check {
		return nil
	}

	objs, err := getKBObjects(o.Client, o.Dynamic, o.Namespace)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "Check whether there are resources left by KubeBlocks before: %s\n", err.Error())
	}

	res := getRemainedResource(objs)
	if len(res) == 0 {
		return nil
	}

	// output remained resource
	var keys []string
	for k := range res {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	resStr := &bytes.Buffer{}
	for _, k := range keys {
		resStr.WriteString(fmt.Sprintf("  %s: %s\n", k, strings.Join(res[k], ",")))
	}
	return fmt.Errorf("there are resources left by previous KubeBlocks version, try to run \"kbcli kubeblocks uninstall\" to clean up\n%s", resStr.String())
}

func (o *InstallOptions) upgrade(cmd *cobra.Command) error {
	// check if KubeBlocks has been installed
	versionInfo, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	v := versionInfo[util.KubeBlocksApp]
	if len(v) > 0 {
		fmt.Fprintln(o.Out, "Current KubeBlocks version "+v)
	} else {
		return errors.New("KubeBlocks does not exits, try to run \"kbcli kubeblocks install\" to install")
	}

	if err = o.preCheck(versionInfo); err != nil {
		return err
	}

	msg := ""
	if len(o.Version) > 0 {
		msg = "to " + o.Version
	}
	spinner := util.Spinner(o.Out, "%-40s", "Upgrading KubeBlocks "+msg)
	defer spinner(false)

	// check whether monitor flag is set by user
	monitorIsSet := false
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "monitor" {
			monitorIsSet = true
		}
	})
	if monitorIsSet {
		o.Sets = append(o.Sets, fmt.Sprintf(kMonitorParam, o.Monitor))
	}

	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}

	// upgrade KubeBlocks chart
	if err = o.upgradeChart(); err != nil {
		return err
	}

	// successfully installed
	spinner(true)

	return nil
}

func (o *InstallOptions) installChart() error {
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
		Timeout:         o.timeout,
	}
	_, err := chart.Install(o.HelmCfg)
	return err
}

func (o *InstallOptions) upgradeChart() error {
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
		Timeout:   o.timeout,
	}
	return chart.Upgrade(o.HelmCfg)
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
    use 'kbcli kubeblocks upgrade --monitor=true' to install later.
`)
	}
}

func (o *InstallOptions) postInstall() error {
	var sets []string
	for _, set := range o.Sets {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	for _, set := range sets {
		if set == "snapshot-controller.enabled=true" {
			if err := o.createVolumeSnapshotClass(); err != nil {
				return err
			}
		}
	}
	// print notes
	if !o.Quiet {
		o.printNotes()
	}
	return nil
}

func (o *InstallOptions) createVolumeSnapshotClass() error {
	options := cluster.CreateVolumeSnapshotClassOptions{}
	options.BaseOptions.Client = o.Dynamic
	options.BaseOptions.IOStreams = o.IOStreams

	spinner := util.Spinner(o.Out, "%-40s", "Configure VolumeSnapshotClass")
	defer spinner(false)

	if err := options.Complete(); err != nil {
		return err
	}
	if err := options.Create(); err != nil {
		return err
	}
	spinner(true)
	return nil
}

func (o *Options) uninstall() error {
	printErr := func(spinner func(result bool), err error) {
		if err == nil || apierrors.IsNotFound(err) ||
			strings.Contains(err.Error(), "release: not found") {
			spinner(true)
			return
		}
		spinner(false)
		fmt.Fprintf(o.Out, "  %s\n", err.Error())
	}

	newSpinner := func(msg string) func(result bool) {
		return util.Spinner(o.Out, fmt.Sprintf("%-50s", msg))
	}

	// uninstall helm release that will delete custom resources, but since finalizers is not empty,
	// custom resources will not be deleted, so we will remove finalizers later.
	v, _ := util.GetVersionInfo(o.Client)
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Namespace: o.Namespace,
	}
	spinner := newSpinner(fmt.Sprintf("Uninstall helm release %s %s", types.KubeBlocksChartName, v[util.KubeBlocksApp]))
	printErr(spinner, chart.Uninstall(o.HelmCfg))

	// remove repo
	spinner = newSpinner("Remove helm repo " + types.KubeBlocksChartName)
	printErr(spinner, helm.RemoveRepo(&repo.Entry{Name: types.KubeBlocksChartName}))

	// get KubeBlocks objects and try to remove them
	objs, err := getKBObjects(o.Client, o.Dynamic, o.Namespace)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "Failed to get KubeBlocks objects %s", err.Error())
	}

	// remove finalizers
	spinner = newSpinner("Remove built-in custom resources")
	printErr(spinner, removeFinalizers(o.Dynamic, objs))

	// delete CRDs
	spinner = newSpinner("Remove custom resource definitions")
	printErr(spinner, deleteCRDs(o.Dynamic, objs.crds))

	_, version, _ := checkIfKubeBlocksInstalled(o.Client)
	// uninstall helm release
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Namespace: o.Namespace,
	}
	spinner = newSpinner(fmt.Sprintf("Uninstall helm release %s %s", types.KubeBlocksChartName, version))
	printErr(spinner, chart.UnInstall(o.HelmCfg))

	// remove repo
	spinner = newSpinner("Remove helm repo " + types.KubeBlocksChartName)
	printErr(spinner, helm.RemoveRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: types.KubeBlocksChartURL}))

	// delete deployments
	spinner = newSpinner("Remove deployments")
	printErr(spinner, deleteDeploys(o.Client, objs.deploys))

	// delete services
	spinner = newSpinner("Remove services")
	printErr(spinner, deleteServices(o.Client, objs.svcs))

	// delete configmaps
	spinner = newSpinner("Remove configmaps")
	printErr(spinner, deleteConfigMaps(o.Client, objs.cms))

	fmt.Fprintln(o.Out, "Uninstall KubeBlocks done")
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
			util.CheckErr(o.Install())
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.postInstall())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana (default true)")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "create the namespace if not present")
	cmd.Flags().BoolVar(&o.check, "check", true, "check kubernetes cluster before install")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "time to wait for installing KubeBlocks")

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
		Example: upgradeExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f, cmd))
			util.CheckErr(o.upgrade(cmd))
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.postInstall())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana")
	cmd.Flags().StringVar(&o.Version, "version", "", "KubeBlocks version")
	cmd.Flags().StringArrayVar(&o.Sets, "set", []string{}, "Set values on the command line (can specify multiple or separate values with commas: key1=val1,key2=val2)")
	cmd.Flags().BoolVar(&o.check, "check", true, "check kubernetes cluster before upgrade")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "time to wait for upgrading KubeBlocks")

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
			util.CheckErr(o.uninstall())
		},
	}
	return cmd
}
