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
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/repo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
)

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
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "Create the namespace if not present")
	cmd.Flags().BoolVar(&o.check, "check", true, "Check kubernetes environment before install")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for installing KubeBlocks")

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

// disableOrEnableSets disable or enable some features according to the kubernetes provider
func (o *InstallOptions) disableOrEnableSets(k8sProvider util.K8sProvider) {
	var sets []string
	for _, set := range o.Sets {
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
		o.Sets = newSets
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
	o.Sets = newSets
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
