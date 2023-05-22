/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package kubeblocks

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/exp/maps"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/spinner"
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
	Timeout   time.Duration
	Wait      bool
}

type InstallOptions struct {
	Options
	Version         string
	Monitor         bool
	Quiet           bool
	CreateNamespace bool
	Check           bool
	ValueOpts       values.Options
}

type addonStatus struct {
	allEnabled  bool
	allDisabled bool
	hasFailed   bool
	outputMsg   string
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

	spinnerMsg = func(format string, a ...any) spinner.Option {
		return spinner.WithMessage(fmt.Sprintf("%-50s", fmt.Sprintf(format, a...)))
	}
)

func newInstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	p := &PreflightOptions{
		PreflightFlags: preflight.NewPreflightFlags(),
		IOStreams:      streams,
	}
	*p.Interactive = false

	cmd := &cobra.Command{
		Use:     "install",
		Short:   "Install KubeBlocks.",
		Args:    cobra.NoArgs,
		Example: installExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.PreCheck())
			util.CheckErr(p.Preflight(f, args, o.ValueOpts))
			util.CheckErr(o.Install())
		},
	}

	cmd.Flags().BoolVar(&o.Monitor, "monitor", true, "Auto install monitoring add-ons including prometheus, grafana and alertmanager-webhook-adaptor")
	cmd.Flags().StringVar(&o.Version, "version", version.DefaultKubeBlocksVersion, "KubeBlocks version")
	cmd.Flags().BoolVar(&o.CreateNamespace, "create-namespace", false, "Create the namespace if not present")
	cmd.Flags().BoolVar(&o.Check, "check", true, "Check kubernetes environment before install")
	cmd.Flags().DurationVar(&o.Timeout, "timeout", 300*time.Second, "Time to wait for installing KubeBlocks, such as --timeout=10m")
	cmd.Flags().BoolVar(&o.Wait, "wait", true, "Wait for KubeBlocks to be ready, including all the auto installed add-ons. It will wait for as long as --timeout")
	cmd.Flags().BoolVar(&p.force, flagForce, p.force, "If present, just print fail item and continue with the following steps")
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)

	return cmd
}

func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error

	// default write log to file
	if err = util.EnableLogToFile(cmd.Flags()); err != nil {
		fmt.Fprintf(o.Out, "Failed to enable the log file %s", err.Error())
	}

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

func (o *InstallOptions) PreCheck() error {
	// check if KubeBlocks has been installed
	v, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	// Todo: KubeBlocks maybe already install but it's status could be Failed.
	// For example: 'kbcli playground init' in windows will fail and try 'kbcli playground init' again immediately,
	// kbcli will output SUCCESSFULLY, however the addon csi is failed and KubeBlocks do not install SUCCESSFULLY
	if v.KubeBlocks != "" {
		printer.Warning(o.Out, "KubeBlocks %s already exists, repeated installation is not supported.\n\n", v.KubeBlocks)
		fmt.Fprintln(o.Out, "If you want to upgrade it, please use \"kbcli kubeblocks upgrade\".")
		return cmdutil.ErrExit
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

	if err = o.checkVersion(v); err != nil {
		return err
	}
	return nil
}

func (o *InstallOptions) Install() error {
	var err error
	// add monitor parameters
	o.ValueOpts.Values = append(o.ValueOpts.Values, fmt.Sprintf(kMonitorParam, o.Monitor))

	// add helm repo
	s := spinner.New(o.Out, spinnerMsg("Add and update repo "+types.KubeBlocksRepoName))
	defer s.Fail()
	// Add repo, if exists, will update it
	if err = helm.AddRepo(&repo.Entry{Name: types.KubeBlocksRepoName, URL: util.GetHelmChartRepoURL()}); err != nil {
		return err
	}
	s.Success()

	// install KubeBlocks
	s = spinner.New(o.Out, spinnerMsg("Install KubeBlocks "+o.Version))
	defer s.Fail()
	if err = o.installChart(); err != nil {
		return err
	}
	s.Success()

	// wait for auto-install addons to be ready
	if err = o.waitAddonsEnabled(); err != nil {
		fmt.Fprintf(o.Out, "Failed to wait for auto-install addons to be enabled, run \"kbcli kubeblocks status\" to check the status\n")
		return err
	}

	if !o.Quiet {
		msg := fmt.Sprintf("\nKubeBlocks %s installed to namespace %s SUCCESSFULLY!\n", o.Version, o.HelmCfg.Namespace())
		if !o.Wait {
			msg = fmt.Sprintf(`
KubeBlocks %s is installing to namespace %s.
You can check the KubeBlocks status by running "kbcli kubeblocks status"
`, o.Version, o.HelmCfg.Namespace())
		}
		fmt.Fprint(o.Out, msg)
		o.printNotes()
	}
	return nil
}

// waitAddonsEnabled waits for auto-install addons status to be enabled
func (o *InstallOptions) waitAddonsEnabled() error {
	if !o.Wait {
		return nil
	}

	addons := make(map[string]*extensionsv1alpha1.Addon)
	fetchAddons := func() error {
		objs, err := o.Dynamic.Resource(types.AddonGVR()).List(context.TODO(), metav1.ListOptions{
			LabelSelector: buildKubeBlocksSelectorLabels(),
		})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if objs == nil || len(objs.Items) == 0 {
			klog.V(1).Info("No Addons found")
			return nil
		}

		for _, obj := range objs.Items {
			addon := &extensionsv1alpha1.Addon{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, addon); err != nil {
				return err
			}

			if addon.Status.ObservedGeneration == 0 {
				klog.V(1).Infof("Addon %s is not observed yet", addon.Name)
				continue
			}

			// addon should be auto installed, check its status
			if addon.Spec.InstallSpec.GetEnabled() {
				addons[addon.Name] = addon
				if addon.Status.Phase != extensionsv1alpha1.AddonEnabled {
					klog.V(1).Infof("Addon %s is not enabled yet, status %s", addon.Name, addon.Status.Phase)
				}
				if addon.Status.Phase == extensionsv1alpha1.AddonFailed {
					klog.V(1).Infof("Addon %s failed:", addon.Name)
					for _, c := range addon.Status.Conditions {
						klog.V(1).Infof("  %s: %s", c.Reason, c.Message)
					}
				}
			}
		}
		return nil
	}

	suffixMsg := func(msg string) string {
		return fmt.Sprintf("%-50s", msg)
	}

	// create spinner
	msg := ""
	header := "Wait for addons to be enabled"
	failedErr := errors.New("there are some addons failed to be enabled")
	s := spinner.New(o.Out, spinnerMsg(header))
	var (
		err         error
		spinnerDone = func() {
			s.SetFinalMsg(msg)
			s.Done("")
			fmt.Fprintln(o.Out)
		}
	)
	// wait all addons to be enabled, or timeout
	if err = wait.PollImmediate(5*time.Second, o.Timeout, func() (bool, error) {
		if err = fetchAddons(); err != nil || len(addons) == 0 {
			return false, err
		}
		status := checkAddons(maps.Values(addons), true)
		msg = suffixMsg(fmt.Sprintf("%s\n  %s", header, status.outputMsg))
		s.SetMessage(msg)
		if status.allEnabled {
			spinnerDone()
			return true, nil
		} else if status.hasFailed {
			return false, failedErr
		}
		return false, nil
	}); err != nil {
		spinnerDone()
		printAddonMsg(o.Out, maps.Values(addons), true)
		return err
	}

	return nil
}

func (o *InstallOptions) checkVersion(v util.Version) error {
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
		fmt.Fprintf(o.ErrOut, "Failed to get resources left by KubeBlocks before: %s\n", err.Error())
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

func (o *InstallOptions) buildChart() *helm.InstallOpts {
	return &helm.InstallOpts{
		Name:            types.KubeBlocksChartName,
		Chart:           types.KubeBlocksChartName + "/" + types.KubeBlocksChartName,
		Wait:            o.Wait,
		Version:         o.Version,
		Namespace:       o.HelmCfg.Namespace(),
		ValueOpts:       &o.ValueOpts,
		TryTimes:        2,
		CreateNamespace: o.CreateNamespace,
		Timeout:         o.Timeout,
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
