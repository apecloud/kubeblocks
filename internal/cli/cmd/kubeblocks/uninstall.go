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
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/strings/slices"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
)

var (
	uninstallExample = templates.Examples(`
		# uninstall KubeBlocks
        kbcli kubeblocks uninstall`)
)

type uninstallOptions struct {
	Options

	// autoApprove if true, skip interactive approval
	autoApprove bool
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &uninstallOptions{
		Options: Options{
			IOStreams: streams,
		},
	}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall KubeBlocks",
		Args:    cobra.NoArgs,
		Example: uninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.preCheck())
			util.CheckErr(o.uninstall())
		},
	}

	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before uninstalling KubeBlocks")
	return cmd
}

func (o *uninstallOptions) preCheck() error {
	printer.Warning(o.Out, "uninstall will remove all KubeBlocks resources.\n")

	// wait user to confirm
	if !o.autoApprove {
		if err := confirmUninstall(o.In); err != nil {
			return err
		}
	}

	preCheckList := []string{
		"clusters.apps.kubeblocks.io",
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

func (o *uninstallOptions) uninstall() error {
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
