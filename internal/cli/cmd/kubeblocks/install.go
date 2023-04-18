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
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/cmd/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/version"
)

const (
	kMonitorParam = "prometheus.enabled=%[1]t,grafana.enabled=%[1]t"
)

type Options struct {
	genericclioptions.IOStreams

	HelmCfg *helm.Config

	// Namespace is the current namespace that the command is running
	Namespace string
	Client    kubernetes.Interface
	Dynamic   dynamic.Interface
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
	# Install KubeBlocks, the default version is same with the kbcli version, the default namespace is kb-system 
	kbcli kubeblocks install
	
	# Install KubeBlocks with specified version
	kbcli kubeblocks install --version=0.4.0

	# Install KubeBlocks with specified namespace, if the namespace is not present, it will be created
	kbcli kubeblocks install --namespace=my-namespace --create-namespace

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

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Auto install monitoring add-ons including prometheus, grafana and alertmanager-webhook-adaptor")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "Create the namespace if not present")
	cmd.Flags().BoolVar(&o.Check, "check", true, "Check kubernetes environment before install")
	cmd.Flags().DurationVar(&o.timeout, "timeout", 1800*time.Second, "Time to wait for installing KubeBlocks")
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
	// to the kb-system namespace
	var targetNamespace string
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		if flag.Name == "namespace" {
			targetNamespace = o.Namespace
		}
	})

	o.HelmCfg = helm.NewConfig(targetNamespace, config, ctx, klog.V(1).Enabled())
	if o.Dynamic, err = f.DynamicClient(); err != nil {
		return err
	}

	o.Client, err = f.KubernetesClientSet()
	return err
}

func (o *InstallOptions) Install() error {
	// check if KubeBlocks has been installed
	v, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	if v.KubeBlocks != "" {
		printer.Warning(o.Out, "KubeBlocks %s already exists, repeated installation is not supported.\n\n", v.KubeBlocks)
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

	if err = o.preCheck(v); err != nil {
		return err
	}

	// add monitor parameters
	o.ValueOpts.Values = append(o.ValueOpts.Values, fmt.Sprintf(kMonitorParam, o.Monitor))

	// add helm repo
	spinner := printer.Spinner(o.Out, "%-50s", "Add and update repo "+types.KubeBlocksRepoName)
	defer spinner(false)
	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksRepoName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	spinner(true)

	// install KubeBlocks chart
	spinner = printer.Spinner(o.Out, "%-50s", "Install KubeBlocks "+o.Version)
	defer spinner(false)
	if err = o.installChart(); err != nil {
		return err
	}
	spinner(true)

	// wait for auto-install addons to be ready
	if err = o.waitAddonsEnabled(); err != nil {
		return err
	}

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

// waitAddonsEnabled waits for auto-install addons status to be enabled
func (o *InstallOptions) waitAddonsEnabled() error {
	addons := make(map[string]bool)
	checkAddons := func() (bool, error) {
		allEnabled := true
		objects, err := o.Dynamic.Resource(types.AddonGVR()).List(context.TODO(), metav1.ListOptions{
			LabelSelector: buildAddonLabelSelector(),
		})
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		if objects == nil || len(objects.Items) == 0 {
			klog.V(1).Info("No Addons found")
			return true, nil
		}

		for _, obj := range objects.Items {
			addon := extensionsv1alpha1.Addon{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &addon); err != nil {
				return false, err
			}

			if addon.Status.ObservedGeneration == 0 {
				klog.V(1).Infof("Addon %s is not observed yet", addon.Name)
				allEnabled = false
				continue
			}

			installable := false
			if addon.Spec.InstallSpec != nil {
				installable = addon.Spec.Installable.AutoInstall
			}

			klog.V(1).Infof("Addon: %s, enabled: %v, status: %s, auto-install: %v",
				addon.Name, addon.Spec.InstallSpec.GetEnabled(), addon.Status.Phase, installable)
			// addon is enabled, then check its status
			if addon.Spec.InstallSpec.GetEnabled() {
				addons[addon.Name] = true
				if addon.Status.Phase != extensionsv1alpha1.AddonEnabled {
					klog.V(1).Infof("Addon %s is not enabled yet", addon.Name)
					addons[addon.Name] = false
					allEnabled = false
				}
			}
		}
		return allEnabled, nil
	}

	okMsg := func(msg string) string {
		return fmt.Sprintf("%-50s %s\n", msg, printer.BoldGreen("OK"))
	}
	failMsg := func(msg string) string {
		return fmt.Sprintf("%-50s %s\n", msg, printer.BoldRed("FAIL"))
	}
	suffixMsg := func(msg string) string {
		return fmt.Sprintf(" %-50s", msg)
	}

	// create spinner
	msg := "Wait for addons to be ready"
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Writer = o.Out
	_ = s.Color("cyan")
	s.Suffix = suffixMsg(msg)
	s.Start()

	var prevUnready []string
	// check addon installing progress
	checkProgress := func() {
		if len(addons) == 0 {
			return
		}
		unready := make([]string, 0)
		ready := make([]string, 0)
		for k, v := range addons {
			if v {
				ready = append(ready, k)
			} else {
				unready = append(unready, k)
			}
		}
		sort.Strings(unready)
		s.Suffix = suffixMsg(fmt.Sprintf("%s\n  %s", msg, strings.Join(unready, "\n  ")))
		for _, r := range ready {
			if !slices.Contains(prevUnready, r) {
				continue
			}
			s.FinalMSG = okMsg("Addon " + r)
			s.Stop()
			s.Suffix = suffixMsg(fmt.Sprintf("%s\n  %s", msg, strings.Join(unready, "\n  ")))
			s.Start()
		}
		prevUnready = unready
	}

	var (
		allEnabled bool
		err        error
	)
	// wait for all auto-install addons to be enabled
	for i := 0; i < viper.GetInt("KB_WAIT_ADDON_TIMES"); i++ {
		allEnabled, err = checkAddons()
		if err != nil {
			s.FinalMSG = failMsg(msg)
			s.Stop()
			return err
		}
		checkProgress()
		if allEnabled {
			s.FinalMSG = okMsg(msg)
			s.Stop()
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	// timeout to wait for all auto-install addons to be enabled
	s.FinalMSG = fmt.Sprintf("%-50s %s\n", msg, printer.BoldRed("TIMEOUT"))
	s.Stop()
	return nil
}

func (o *InstallOptions) preCheck(v util.Version) error {
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
	k8sVersionStr := v.Kubernetes
	if k8sVersionStr == "" {
		return versionErr
	}

	semVer := util.GetK8sSemVer(k8sVersionStr)
	if len(semVer) == 0 {
		return versionErr
	}

	// output kubernetes version
	fmt.Fprintf(o.Out, "Kubernetes version %s\n", ""+semVer)

	// disable or enable some features according to the kubernetes environment
	provider, err := util.GetK8sProvider(k8sVersionStr, o.Client)
	if err != nil {
		return fmt.Errorf("failed to get kubernetes provider: %v", err)
	}
	if provider.IsCloud() {
		fmt.Fprintf(o.Out, "Kubernetes provider %s\n", provider)
	}

	// check kbcli version, now do nothing
	fmt.Fprintf(o.Out, "kbcli version %s\n", v.Cli)

	return nil
}

func (o *InstallOptions) checkNamespace() error {
	// target namespace is not specified, use default namespace
	if o.HelmCfg.Namespace() == "" {
		o.HelmCfg.SetNamespace(types.DefaultNamespace)
		o.CreateNamespace = true
		fmt.Fprintf(o.Out, "KubeBlocks will be installed to namespace \"%s\"\n", o.HelmCfg.Namespace())
	}

	// check if namespace exists
	if !o.CreateNamespace {
		_, err := o.Client.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{})
		return err
	}
	return nil
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
-> To view the monitoring add-ons web console:
    kbcli dashboard list        # list all monitoring web consoles
    kbcli dashboard open <name> # open the web console in the default browser
`)
	} else {
		fmt.Fprint(o.Out, `
Note: Monitoring add-ons are not installed.
    Use 'kbcli addon enable <addon-name>' to install them later.
`)
	}
}

func (o *InstallOptions) createVolumeSnapshotClass() error {
	createFunc := func() error {
		options := cluster.CreateVolumeSnapshotClassOptions{}
		options.BaseOptions.Dynamic = o.Dynamic
		options.BaseOptions.IOStreams = o.IOStreams
		options.BaseOptions.Quiet = true

		spinner := printer.Spinner(o.Out, "%-50s", "Configure VolumeSnapshotClass")
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
