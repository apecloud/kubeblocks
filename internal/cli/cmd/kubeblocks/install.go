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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

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

var (
	// unsupported sets in non-cloud kubernetes cluster
	disabledSetsInLocalK8s = [...]string{"loadbalancer.enabled"}

	// enabled sets in cloud kubernetes
	enabledSetsInCloudK8s = [...]string{"snapshot-controller.enabled"}
)

type Options struct {
	genericclioptions.IOStreams

	HelmCfg *helm.Config

	// Namespace is the current namespace that the command is running
	Namespace string
	Client    kubernetes.Interface
	Dynamic   dynamic.Interface
	verbose   bool
}

type InstallOptions struct {
	Options
	Version         string
	Monitor         bool
	Quiet           bool
	CreateNamespace bool
	Check           bool
	ValueOpts       values.Options
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
)

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install KubeBlocks.",
		Args:    cobra.NoArgs,
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Install())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Set monitor enabled and install Prometheus, AlertManager and Grafana (default true)")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "Create the namespace if not present")
	cmd.Flags().BoolVar(&o.Check, "check", true, "Check kubernetes environment before install")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for installing KubeBlocks")
	cmd.Flags().BoolVar(&o.verbose, "verbose", false, "Show logs in detail.")
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)

	return cmd
}

func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
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

	// check whether --namespace is specified, if not, KubeBlocks will be installed
	// to a default namespace
	var targetNamespace string
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "namespace" {
			targetNamespace = o.Namespace
		}
	})

	o.HelmCfg = helm.NewConfig(targetNamespace, config, ctx, o.verbose)
	if o.Dynamic, err = f.DynamicClient(); err != nil {
		return err
	}

	o.Client, err = f.KubernetesClientSet()
	return err
}

func (o *InstallOptions) Install() error {
	// check if KubeBlocks has been installed
	versionInfo, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	if v := versionInfo[util.KubeBlocksApp]; len(v) > 0 {
		printer.Warning(o.Out, "KubeBlocks %s already exists, repeated installation is not supported.\n\n", v)
		fmt.Fprintln(o.Out, "If you want to upgrade it, please use \"kbcli kubeblocks upgrade\".")
		return nil
	}

	// check whether the namespace exists
	if err = o.checkNamespace(); err != nil {
		return err
	}

	// check whether there are remained resource left by previous KubeBlocks installation, if yes,
	// output the resource name
	if err = o.checkRemainedResource(); err != nil {
		return err
	}

	if err = o.preCheck(versionInfo); err != nil {
		return err
	}

	// add monitor parameters
	o.ValueOpts.Values = append(o.ValueOpts.Values, fmt.Sprintf(kMonitorParam, o.Monitor))

	// add helm repo
	spinner := util.Spinner(o.Out, "%-40s", "Add and update repo "+types.KubeBlocksChartName)
	defer spinner(false)
	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksChartName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	spinner(true)

	// install KubeBlocks chart
	spinner = util.Spinner(o.Out, "%-40s", "Install KubeBlocks "+o.Version)
	defer spinner(false)
	if err = o.installChart(); err != nil {
		return err
	}
	spinner(true)

	// create VolumeSnapshotClass
	if err = o.createVolumeSnapshotClass(); err != nil {
		return err
	}

	if !o.Quiet {
		fmt.Fprintf(o.Out, "\nKubeBlocks %s installed to namespace %s SUCCESSFULLY!\n",
			o.Version, o.HelmCfg.Namespace())
		o.printNotes()
	}
	return nil
}

func (o *InstallOptions) preCheck(versionInfo map[util.AppName]string) error {
	if !o.Check {
		return nil
	}

	// check installing version exists
	if exists, err := versionExists(o.Version); !exists {
		if err != nil {
			klog.V(1).Infof(err.Error())
		}
		return fmt.Errorf("version %s does not exist, please use \"kbcli kubeblocks list-versions --devel\" to show the available versions", o.Version)
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
		return fmt.Errorf("kubernetes version should be greater than or equal to %s", requiredK8sVersion)
	}

	// check kbcli version, now do nothing
	spinner = util.Spinner(o.Out, "%-40s", "kbcli version "+versionInfo[util.KBCLIApp])
	spinner(true)

	// disable or enable some features according to the kubernetes environment
	provider := util.GetK8sProvider(k8sVersionStr)
	if provider.IsCloud() {
		spinner = util.Spinner(o.Out, "%-40s", "Kubernetes provider "+provider)
		spinner(true)
	}
	o.disableOrEnableSets(provider)

	return nil
}

func (o *InstallOptions) checkNamespace() error {
	// target namespace is not specified, use default namespace
	if o.HelmCfg.Namespace() == "" {
		o.HelmCfg.SetNamespace(types.DefaultNamespace)
		o.CreateNamespace = true
		fmt.Fprintf(o.Out, "KubeBlocks will be installed to namespace \"%s\".\n", o.HelmCfg.Namespace())
	}

	// check if namespace exists
	if !o.CreateNamespace {
		_, err := o.Client.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{})
		return err
	}
	return nil
}

// disableOrEnableSets disable or enable some features according to the kubernetes provider
func (o *InstallOptions) disableOrEnableSets(k8sProvider util.K8sProvider) {
	var sets []string
	for _, set := range o.ValueOpts.Values {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}

	switch k8sProvider {
	case util.EKSProvider:
		// some features must be enabled on cloud kubernetes environment, if they are disabled,
		// turn on these features and output message
		o.enableSets(sets)
	case util.UnknownProvider:
		// check whether user turn on features that only enable on cloud kubernetes cluster,
		// if yes, turn off these features and output message
		o.disableSets(sets)
	}
}

func (o *InstallOptions) enableSets(sets []string) {
	var (
		newSets     []string
		removedSets []string
	)

	// check all sets, remove sets that disable the features that must be enabled
	for _, set := range sets {
		need := true
		for _, key := range enabledSetsInCloudK8s {
			if !strings.Contains(set, key) {
				continue
			}

			// found unsupported, parse its value
			kv := strings.Split(set, "=")
			if len(kv) <= 1 {
				break
			}

			// whether it is true or false, just remove it, we will add all enabled sets later
			need = false

			// if value is false, record it
			if val, _ := strconv.ParseBool(kv[1]); !val {
				removedSets = append(removedSets, set)
				break
			}
		}
		if need {
			newSets = append(newSets, set)
		}
	}

	// add enabled sets
	for _, key := range enabledSetsInCloudK8s {
		newSets = append(newSets, key+"=true")
	}

	if len(removedSets) == 0 {
		o.ValueOpts.Values = newSets
		return
	}

	msg := "following parameters must be enabled in current kubernetes environment, they will be enabled\n"
	if len(removedSets) == 1 {
		msg = "following parameter must be enabled in current kubernetes environment, it will be enabled\n"
	}
	printer.Warning(o.Out, msg)
	for _, set := range removedSets {
		fmt.Fprintf(o.Out, "  · %s\n", set)
	}
	o.ValueOpts.Values = newSets
}

func (o *InstallOptions) disableSets(sets []string) {
	var (
		newSets      []string
		disabledSets []string
	)

	// check all sets, remove unsupported and output message
	for _, set := range sets {
		need := true
		for _, key := range disabledSetsInLocalK8s {
			if !strings.Contains(set, key) {
				continue
			}

			// found unsupported, parse its value
			kv := strings.Split(set, "=")
			if len(kv) <= 1 {
				break
			}

			// if value is false, ignore it
			if val, _ := strconv.ParseBool(kv[1]); !val {
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

	msg := "following parameters are not available in current kubernetes environment, they will be disabled\n"
	if len(disabledSets) == 1 {
		msg = "following parameter is not available in current kubernetes environment, it will be disabled\n"
	}
	printer.Warning(o.Out, msg)
	for _, set := range disabledSets {
		fmt.Fprintf(o.Out, "  · %s\n", set)
	}
	o.ValueOpts.Values = newSets
}

func (o *InstallOptions) checkRemainedResource() error {
	if !o.Check {
		return nil
	}

	ns, _ := util.GetKubeBlocksNamespace(o.Client)
	if ns == "" {
		ns = o.Namespace
	}

	// Now, we only check whether there are resources left by KubeBlocks, ignore
	// the addon resources.
	objs, err := getKBObjects(o.Dynamic, ns, nil)
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

func (o *InstallOptions) installChart() error {
	_, err := o.buildChart().Install(o.HelmCfg)
	return err
}

func (o *InstallOptions) printNotes() {
	fmt.Fprintf(o.Out, `
-> Basic commands for cluster:
    kbcli cluster create -h     # help information about creating a database cluster
    kbcli cluster list          # list all database clusters
    kbcli cluster describe <cluster name>  # get cluster information

-> Uninstall KubeBlocks:
    kbcli kubeblocks uninstall
`)
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

func (o *InstallOptions) createVolumeSnapshotClass() error {
	createFunc := func() error {
		options := cluster.CreateVolumeSnapshotClassOptions{}
		options.BaseOptions.Dynamic = o.Dynamic
		options.BaseOptions.IOStreams = o.IOStreams
		options.BaseOptions.Quiet = true

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

	var sets []string
	for _, set := range o.ValueOpts.Values {
		splitSet := strings.Split(set, ",")
		sets = append(sets, splitSet...)
	}
	for _, set := range sets {
		if set != "snapshot-controller.enabled=true" {
			continue
		}

		if err := createFunc(); err != nil {
			return err
		} else {
			// only need to create once
			return nil
		}
	}
	return nil
}

func (o *InstallOptions) buildChart() *helm.InstallOpts {
	return &helm.InstallOpts{
		Name:            types.KubeBlocksChartName,
		Chart:           types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:            true,
		Version:         o.Version,
		Namespace:       o.HelmCfg.Namespace(),
		ValueOpts:       &o.ValueOpts,
		TryTimes:        2,
		CreateNamespace: o.CreateNamespace,
		Timeout:         o.timeout,
		Atomic:          true,
	}
}

func versionExists(version string) (bool, error) {
	if version == "" {
		return true, nil
	}

	allVers, err := getHelmChartVersions(types.KubeBlocksChartName)
	if err != nil {
		return false, err
	}

	for _, v := range allVers {
		if v.String() == version {
			return true, nil
		}
	}
	return false, nil
}
