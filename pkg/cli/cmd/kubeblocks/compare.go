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
	"fmt"

	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/pkg/cli/types"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
	"github.com/apecloud/kubeblocks/pkg/cli/util/helm"
)

var (
	diffExample = templates.Examples(`
	# compare installed KubeBlocks with specified version
	kbcli kubeblocks compare 0.4.0

	# compare two specified KubeBlocks version
	kbcli kubeblocks compare 0.4.0 0.5.0`)
)

func newCompareCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &InstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}
	var showDetail bool
	cmd := &cobra.Command{
		Use:     "compare version [OTHER-VERSION]",
		Short:   "List the changes between two different version KubeBlocks.",
		Args:    cobra.MaximumNArgs(2),
		Example: diffExample,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) >= 2 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			allVers, _ := getHelmChartVersions(types.KubeBlocksChartName)
			var names []string
			for _, v := range allVers {
				names = append(names, v.String())
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.compare(args, showDetail))
		},
	}
	cmd.Flags().BoolVar(&showDetail, "details", false, "show the different details between two kubeblocks version")
	return cmd
}

// validateCompareVersion validate the user inputs version and save the valid value into versionA and versionB
func (o *InstallOptions) validateCompareVersion(args []string, versionA, versionB *string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("need specify at least one version to compare")
	case 1:
		v, err := util.GetVersionInfo(o.Client)
		if err != nil {
			return err
		}
		if v.KubeBlocks == "" {
			return fmt.Errorf("KubeBlocks does not exist, please install it first")
		}
		if args[0] == v.KubeBlocks {
			return fmt.Errorf("input version %s is same with the current version, no need to compare", args[0])
		}
		*versionA, *versionB = v.KubeBlocks, args[0]
	default:
		if args[0] == args[1] {
			return fmt.Errorf("input version %s and %s are same, no need to compare", args[0], args[0])
		}
		*versionA, *versionB = args[0], args[1]
	}
	return nil
}

func (o *InstallOptions) compare(args []string, detail bool) error {
	var versionA, versionB string
	if err := o.validateCompareVersion(args, &versionA, &versionB); err != nil {
		return err
	}
	// update repo
	if err := helm.AddRepo(&repo.Entry{Name: types.KubeBlocksRepoName, URL: util.GetHelmChartRepoURL()}); err != nil {

		return fmt.Errorf(err.Error())
	}
	// check version is available
	if exists, err := versionExists(versionA); !exists {
		if err != nil {
			return err
		}
		return fmt.Errorf("version %s does not exist, please use \"kbcli kubeblocks list-versions --devel\" to show the available versions", versionA)
	}

	if exists, err := versionExists(versionB); !exists {
		if err != nil {
			return err
		}
		return fmt.Errorf("version %s does not exist, please use \"kbcli kubeblocks list-versions --devel\" to show the available versions", versionB)
	}
	return o.showDiff(versionA, versionB, detail)
}

// buildTemplate builds `helm template` InstallOpts for KubeBlocks
func (o *InstallOptions) buildTemplate(version string) *helm.InstallOpts {
	ops := helm.GetTemplateInstallOps(types.KubeBlocksChartName, fmt.Sprintf("%s/%s", types.KubeBlocksChartName, types.KubeBlocksChartName), version, o.Namespace)
	ops.Wait = o.Wait
	ops.ValueOpts = &o.ValueOpts
	ops.CreateNamespace = o.CreateNamespace
	ops.Timeout = o.Timeout
	return ops
}

func (o *InstallOptions) showDiff(version1, version2 string, detail bool) error {
	// use `helm template` get the chart manifest
	helmInstallOpts := o.buildTemplate(version1)
	releaseA, err := helmInstallOpts.Install(helm.NewFakeConfig(o.Namespace))
	if err != nil {
		return err
	}
	helmInstallOpts.Version = version2
	releaseB, err := helmInstallOpts.Install(helm.NewFakeConfig(o.Namespace))
	if err != nil {
		return err
	}
	return helm.OutputDiff(releaseA, releaseB, version1, version2, o.Out, detail)
}
