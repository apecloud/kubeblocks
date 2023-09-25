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
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/spinner"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

var (
	upgradeExample = templates.Examples(`
	# Upgrade KubeBlocks to specified version
	kbcli kubeblocks upgrade --version=0.4.0

	# Upgrade KubeBlocks other settings, for example, set replicaCount to 3
	kbcli kubeblocks upgrade --set replicaCount=3`)
)

func newUpgradeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrade KubeBlocks.",
		Args:    cobra.NoArgs,
		Example: upgradeExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.Upgrade())
		},
	}

	cmd.Flags().StringVar(&o.Version, "version", "", "Set KubeBlocks version")
	cmd.Flags().BoolVar(&o.Check, "check", true, "Check kubernetes environment before upgrade")
	cmd.Flags().DurationVar(&o.Timeout, "timeout", 300*time.Second, "Time to wait for upgrading KubeBlocks, such as --timeout=10m")
	cmd.Flags().BoolVar(&o.Wait, "wait", true, "Wait for KubeBlocks to be ready. It will wait for a --timeout period")
	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before upgrading KubeBlocks")
	helm.AddValueOptionsFlags(cmd.Flags(), &o.ValueOpts)

	return cmd
}

func (o *InstallOptions) Upgrade() error {
	if o.HelmCfg.Namespace() == "" {
		ns, err := util.GetKubeBlocksNamespace(o.Client)
		if err != nil || ns == "" {
			printer.Warning(o.Out, "Failed to find deployed KubeBlocks.\n\n")
			fmt.Fprint(o.Out, "Use \"kbcli kubeblocks install\" to install KubeBlocks.\n")
			fmt.Fprintf(o.Out, "Use \"kbcli kubeblocks status\" to get information in more details.\n")
			return nil
		}
		o.HelmCfg.SetNamespace(ns)
	}

	// check flags already been set
	if o.Version == "" && helm.ValueOptsIsEmpty(&o.ValueOpts) {
		fmt.Fprint(o.Out, "Nothing to upgrade, --set, --version should be specified.\n")
		return nil
	}

	// check if KubeBlocks has been installed
	v, err := util.GetVersionInfo(o.Client)
	if err != nil {
		return err
	}

	kbVersion := v.KubeBlocks
	if kbVersion == "" {
		return errors.New("KubeBlocks does not exist, try to run \"kbcli kubeblocks install\" to install")
	}

	if kbVersion == o.Version && helm.ValueOptsIsEmpty(&o.ValueOpts) {
		fmt.Fprintf(o.Out, "Current version %s is same with the upgraded version, no need to upgrade.\n", o.Version)
		return nil
	}
	fmt.Fprintf(o.Out, "Current KubeBlocks version %s.\n", v.KubeBlocks)

	if err = o.checkVersion(v); err != nil {
		return err
	}

	// double check for KubeBlocks upgrade
	// and only check when KubeBlocks version change
	if !o.autoApprove && o.Version != "" {
		oldVersion, err := version.NewVersion(kbVersion)
		if err != nil {
			return err
		}
		newVersion, err := version.NewVersion(o.Version)
		if err != nil {
			return err
		}
		upgradeWarn := ""
		if oldVersion.GreaterThan(newVersion) {
			upgradeWarn = printer.BoldYellow(fmt.Sprintf("Warning: You're attempting to downgrade KubeBlocks version from %s to %s, this action may cause your clusters and some KubeBlocks feature unavailable.\nEnsure you proceed after reviewing detailed release notes at https://github.com/apecloud/kubeblocks/releases.", kbVersion, o.Version))
		} else {
			upgradeWarn = fmt.Sprintf("Upgrade KubeBlocks from %s to %s", kbVersion, o.Version)
		}

		if err = prompt.Confirm(nil, o.In, upgradeWarn, "Please type 'Yes/yes' to confirm your operation:"); err != nil {
			return err
		}
	}

	// add helm repo
	s := spinner.New(o.Out, spinnerMsg("Add and update repo "+types.KubeBlocksChartName))
	defer s.Fail()
	// Add repo, if exists, will update it
	if err = helm.AddRepo(newHelmRepoEntry()); err != nil {
		return err
	}
	s.Success()

	// stop the old version KubeBlocks, otherwise the old version KubeBlocks will reconcile the
	// new version resources, which may not be compatible. helm will start the new version
	// KubeBlocks after upgrade.
	s = spinner.New(o.Out, spinnerMsg("Stop KubeBlocks "+kbVersion))
	defer s.Fail()
	if err = o.stopKubeBlocks(); err != nil {
		return err
	}
	s.Success()

	// it's time to upgrade
	msg := ""
	if o.Version != "" {
		msg = "to " + o.Version
	}
	s = spinner.New(o.Out, spinnerMsg("Upgrading KubeBlocks "+msg))
	defer s.Fail()
	// upgrade KubeBlocks chart
	if err = o.upgradeChart(); err != nil {
		return err
	}
	// successfully upgraded
	s.Success()

	if !o.Quiet {
		fmt.Fprintf(o.Out, "\nKubeBlocks has been upgraded %s SUCCESSFULLY!\n", msg)
		o.printNotes()
	}
	return nil
}

func (o *InstallOptions) upgradeChart() error {
	return o.buildChart().Upgrade(o.HelmCfg)
}

// stopKubeBlocks stops the old version KubeBlocks by setting the replicas of
// KubeBlocks deployment to 0
func (o *InstallOptions) stopKubeBlocks() error {
	kbDeploy, err := util.GetKubeBlocksDeploy(o.Client)
	if err != nil {
		return err
	}

	// if KubeBlocks is not deployed, just return
	if kbDeploy == nil {
		klog.V(1).Info("KubeBlocks is not deployed, no need to stop")
		return nil
	}

	if _, err = o.Client.AppsV1().Deployments(kbDeploy.Namespace).Patch(
		context.TODO(), kbDeploy.Name, apitypes.JSONPatchType,
		[]byte(`[{"op": "replace", "path": "/spec/replicas", "value": 0}]`),
		metav1.PatchOptions{}); err != nil {
		return err
	}

	// wait for KubeBlocks to be stopped
	return wait.PollImmediate(5*time.Second, o.Timeout, func() (bool, error) {
		kbDeploy, err = util.GetKubeBlocksDeploy(o.Client)
		if err != nil {
			return false, err
		}
		if *kbDeploy.Spec.Replicas == 0 && kbDeploy.Status.Replicas == 0 &&
			kbDeploy.Status.AvailableReplicas == 0 {
			return true, nil
		}
		return false, nil
	})
}
