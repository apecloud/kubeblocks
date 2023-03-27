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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"helm.sh/helm/v3/pkg/repo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/utils/strings/slices"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/helm"
	"github.com/apecloud/kubeblocks/internal/constant"
)

var (
	uninstallExample = templates.Examples(`
		# uninstall KubeBlocks
        kbcli kubeblocks uninstall`)
)

type uninstallOptions struct {
	factory cmdutil.Factory
	Options

	// autoApprove if true, skip interactive approval
	autoApprove     bool
	removePVs       bool
	removePVCs      bool
	removeNamespace bool
	addons          []*extensionsv1alpha1.Addon
}

func newUninstallCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &uninstallOptions{
		Options: Options{
			IOStreams: streams,
		},
		factory: f,
	}
	cmd := &cobra.Command{
		Use:     "uninstall",
		Short:   "Uninstall KubeBlocks.",
		Args:    cobra.NoArgs,
		Example: uninstallExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Complete(f, cmd))
			util.CheckErr(o.preCheck())
			util.CheckErr(o.uninstall())
		},
	}

	cmd.Flags().BoolVar(&o.autoApprove, "auto-approve", false, "Skip interactive approval before uninstalling KubeBlocks")
	cmd.Flags().BoolVar(&o.removePVs, "remove-pvs", false, "Remove PersistentVolume or not")
	cmd.Flags().BoolVar(&o.removePVCs, "remove-pvcs", false, "Remove PersistentVolumeClaim or not")
	cmd.Flags().BoolVar(&o.removeNamespace, "remove-namespace", false, "Remove default created \"kb-system\" namespace or not")
	return cmd
}

func (o *uninstallOptions) preCheck() error {
	// wait user to confirm
	if !o.autoApprove {
		printer.Warning(o.Out, "uninstall will remove all KubeBlocks resources.\n")
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
		if strings.Contains(crd.GetName(), constant.APIGroup) &&
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

	// verify where kubeblocks is installed
	kbNamespace, err := util.GetKubeBlocksNamespace(o.Client)
	if err != nil {
		printer.Warning(o.Out, "failed to locate KubeBlocks meta, will clean up all KubeBlocks resources.\n")
	} else if o.Namespace != kbNamespace {
		o.Namespace = kbNamespace
		fmt.Fprintf(o.Out, "Uninstall KubeBlocks in namespace \"%s\"\n", kbNamespace)
	}
	return nil
}

func (o *uninstallOptions) uninstall() error {
	printSpinner := func(spinner func(result bool), err error) {
		if err == nil || apierrors.IsNotFound(err) ||
			strings.Contains(err.Error(), "release: not found") {
			spinner(true)
			return
		}
		spinner(false)
		fmt.Fprintf(o.Out, "  %s\n", err.Error())
	}
	newSpinner := func(msg string) func(result bool) {
		return printer.Spinner(o.Out, fmt.Sprintf("%-50s", msg))
	}

	// uninstall all KubeBlocks addons
	printSpinner(newSpinner("Uninstall KubeBlocks addons"), o.uninstallAddons())

	// uninstall helm release that will delete custom resources, but since finalizers is not empty,
	// custom resources will not be deleted, so we will remove finalizers later.
	v, _ := util.GetVersionInfo(o.Client)
	chart := helm.InstallOpts{
		Name:      types.KubeBlocksChartName,
		Namespace: o.Namespace,
	}
	printSpinner(newSpinner("Uninstall helm release "+types.KubeBlocksChartName+" "+v[util.KubeBlocksApp]),
		chart.Uninstall(o.HelmCfg))

	// remove repo
	printSpinner(newSpinner("Remove helm repo "+types.KubeBlocksChartName),
		helm.RemoveRepo(&repo.Entry{Name: types.KubeBlocksChartName}))

	// get KubeBlocks objects, then try to remove them
	objs, err := getKBObjects(o.Dynamic, o.Namespace, o.addons)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "Failed to get KubeBlocks objects %s", err.Error())
	}

	// remove finalizers of custom resources, then that will be deleted
	printSpinner(newSpinner("Remove built-in custom resources"), removeCustomResources(o.Dynamic, objs))

	var gvrs []schema.GroupVersionResource
	for k := range objs {
		gvrs = append(gvrs, k)
	}
	sort.SliceStable(gvrs, func(i, j int) bool {
		g1 := gvrs[i]
		g2 := gvrs[j]
		return strings.Compare(g1.Resource, g2.Resource) < 0
	})

	for _, gvr := range gvrs {
		if gvr == types.PVCGVR() && !o.removePVCs {
			continue
		}
		if gvr == types.PVGVR() && !o.removePVs {
			continue
		}
		if v, ok := objs[gvr]; !ok || len(v.Items) == 0 {
			continue
		}
		printSpinner(newSpinner("Remove "+gvr.Resource), deleteObjects(o.Dynamic, gvr, objs[gvr]))
	}

	// delete namespace if it is default namespace
	if o.Namespace == types.DefaultNamespace && o.removeNamespace {
		printSpinner(newSpinner("Remove namespace "+types.DefaultNamespace),
			deleteNamespace(o.Client, types.DefaultNamespace))
	}

	fmt.Fprintln(o.Out, "Uninstall KubeBlocks done.")
	return nil
}

// uninstallAddons uninstall all KubeBlocks addons
func (o *uninstallOptions) uninstallAddons() error {
	var (
		allErrs []error
		stop    bool
		err     error
	)
	uninstallAddon := func(addon *extensionsv1alpha1.Addon) error {
		klog.V(1).Infof("Uninstall %s", addon.Name)
		if _, err := o.Dynamic.Resource(types.AddonGVR()).Patch(context.TODO(), addon.Name, k8sapitypes.JSONPatchType,
			[]byte("[{\"op\": \"replace\", \"path\": \"/spec/install/enabled\", \"value\": false }]"),
			metav1.PatchOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	processAddons := func(processFn func(addon *extensionsv1alpha1.Addon) error) ([]*extensionsv1alpha1.Addon, error) {
		var addons []*extensionsv1alpha1.Addon
		objects, err := o.Dynamic.Resource(types.AddonGVR()).List(context.TODO(), metav1.ListOptions{
			LabelSelector: buildAddonLabelSelector(),
		})
		if err != nil && !apierrors.IsNotFound(err) {
			klog.V(1).Infof("Failed to get KubeBlocks addons %s", err.Error())
			allErrs = append(allErrs, err)
			return nil, utilerrors.NewAggregate(allErrs)
		}
		if objects == nil {
			return nil, nil
		}

		// if all addons are disabled, then we will stop uninstalling addons
		stop = true
		for _, obj := range objects.Items {
			addon := extensionsv1alpha1.Addon{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &addon); err != nil {
				klog.V(1).Infof("Failed to convert KubeBlocks addon %s", err.Error())
				allErrs = append(allErrs, err)
				continue
			}
			klog.V(1).Infof("Addon: %s, enabled: %v, status: %s",
				addon.Name, addon.Spec.InstallSpec.GetEnabled(), addon.Status.Phase)
			addons = append(addons, &addon)
			if addon.Status.Phase == extensionsv1alpha1.AddonDisabled {
				continue
			}
			// if there is an enabled addon, then we will continue uninstalling addons
			// and wait for a while to make sure all addons are disabled
			stop = false
			if processFn == nil {
				continue
			}
			if err = processFn(&addon); err != nil && !apierrors.IsNotFound(err) {
				klog.V(1).Infof("Failed to uninstall KubeBlocks addon %s", err.Error())
				allErrs = append(allErrs, err)
			}
		}
		return addons, utilerrors.NewAggregate(allErrs)
	}

	// get all addons and uninstall them
	if o.addons, err = processAddons(uninstallAddon); err != nil {
		return err
	}

	if len(o.addons) == 0 || stop {
		return nil
	}

	// check if all addons are disabled, if so, then we will stop checking addons
	// status otherwise, we will wait for a while and check again
	for i := 0; i < viper.GetInt("KB_WAIT_ADDON_TIMES"); i++ {
		klog.V(1).Infof("Wait for %d seconds and check addons disabled again", 5)
		time.Sleep(5 * time.Second)
		// pass a nil processFn, we will only check addons status, do not try to
		// uninstall addons again
		if o.addons, err = processAddons(nil); err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
	if !stop {
		allErrs = append(allErrs, fmt.Errorf("failed to uninstall KubeBlocks addons"))
	}
	return utilerrors.NewAggregate(allErrs)
}
