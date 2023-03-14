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

package addon

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	discoverycli "k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"
)

type addonEnableFlags struct {
	MemorySets       []string
	CPUSets          []string
	StorageSets      []string
	ReplicaCountSets []string
	StorageClassSets []string
	TolerationsSet   []string
}

func (r *addonEnableFlags) useDefault() bool {
	return len(r.MemorySets) == 0 &&
		len(r.CPUSets) == 0 &&
		len(r.StorageSets) == 0 &&
		len(r.ReplicaCountSets) == 0 &&
		len(r.StorageClassSets) == 0 &&
		len(r.TolerationsSet) == 0
}

type addonCmdOpts struct {
	genericclioptions.IOStreams

	Factory cmdutil.Factory
	dynamic dynamic.Interface

	addon extensionsv1alpha1.Addon

	*patch.Options
	addonEnableFlags *addonEnableFlags

	complete func(self *addonCmdOpts, cmd *cobra.Command, args []string) error
}

// NewAddonCmd for addon functions
func NewAddonCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "Addon command.",
	}
	cmd.AddCommand(
		newListCmd(f, streams),
		newDescribeCmd(f, streams),
		newEnableCmd(f, streams),
		newDisableCmd(f, streams),
	)
	return cmd
}

func newListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.AddonGVR())
	cmd := &cobra.Command{
		Use:               "list ",
		Short:             "List addons.",
		Aliases:           []string{"ls"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(addonListRun(o))
		},
	}
	o.AddFlags(cmd)
	return cmd
}

func newDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addonCmdOpts{
		Options:   patch.NewOptions(f, streams, types.AddonGVR()),
		Factory:   f,
		IOStreams: streams,
		complete:  addonDescribeHandler,
	}
	cmd := &cobra.Command{
		Use:               "describe ADDON_NAME",
		Short:             "Describe an addon specification.",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.AddonGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.init(args))
			util.CheckErr(o.fetchAddonObj())
			util.CheckErr(o.complete(o, cmd, args))
		},
	}
	return cmd
}

func newEnableCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addonCmdOpts{
		Options:          patch.NewOptions(f, streams, types.AddonGVR()),
		Factory:          f,
		IOStreams:        streams,
		addonEnableFlags: &addonEnableFlags{},
		complete:         addonEnableDisableHandler,
	}

	// # kbcli addon enable flags:
	// # [--memory [extraName:]<request>/<limit> (can specify multiple if has extra items)]
	// # [--cpu [extraName:]<request>/<limit> (can specify multiple if has extra items)]
	// # [--storage [extraName:]<request> (can specify multiple if has extra items)]
	// # [--replicas [extraName:]<number> (can specify multiple if has extra items)]
	// # [--storage-class [extraName:]<storage class name> (can specify multiple if has extra items)]
	// # [--tolerations [extraName:]<toleration JSON list items> (can specify multiple if has extra items)]
	// # [--dry-run] # TODO

	cmd := &cobra.Command{
		Use:               "enable ADDON_NAME",
		Short:             "Enable an addon.",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.AddonGVR()),
		Example: templates.Examples(`
    	# Enabled "prometheus" addon
    	kbcli addon enable prometheus
    
        # Enabled "prometheus" addon with custom resources settings
    	kbcli addon enable prometheus --memory 512Mi/4Gi --storage 8Gi --replicas 2
    
        # Enabled "prometheus" addon and its extra alertmanager component with custom resources settings 
    	kbcli addon enable prometheus --memory 512Mi/4Gi --storage 8Gi --replicas 2 \
  			--memory alertmanager:16Mi/256Mi --storage: alertmanager:1Gi --replicas alertmanager:2 

        # Enabled "prometheus" addon with tolerations 
    	kbcli addon enable prometheus --tolerations '[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]' \
			--tolerations 'alertmanager:[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]'`),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.init(args))
			util.CheckErr(o.fetchAddonObj())
			util.CheckErr(o.complete(o, cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.MemorySets, "memory", []string{},
		"Sets addon memory resource values (--memory [extraName:]<request>/<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.CPUSets, "cpu", []string{},
		"Sets addon CPU resource values (--cpu [extraName:]<request>/<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.StorageSets, "storage", []string{},
		"Sets addon storage size (--storage [extraName:]<request>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.ReplicaCountSets, "replicas", []string{},
		"Sets addon component replica count (--replicas [extraName:]<number>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.StorageClassSets, "storage-class", []string{},
		"Sets addon storage class name (--storage-class [extraName:]<storage class name>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.TolerationsSet, "tolerations", []string{},
		"Sets addon pod tolerations (--tolerations [extraName:]<toleration JSON list items>) (can specify multiple if has extra items))")

	o.Options.AddFlags(cmd)
	return cmd
}

func newDisableCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addonCmdOpts{
		Options:   patch.NewOptions(f, streams, types.AddonGVR()),
		Factory:   f,
		IOStreams: streams,
		complete:  addonEnableDisableHandler,
	}
	cmd := &cobra.Command{
		Use:               "disable ADDON_NAME",
		Short:             "Disable an addon.",
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.AddonGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.init(args))
			util.CheckErr(o.fetchAddonObj())
			util.CheckErr(o.complete(o, cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.Options.AddFlags(cmd)
	return cmd
}

func (o *addonCmdOpts) init(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing addon name")
	}
	if len(args) > 1 {
		return fmt.Errorf("only accept enable/disable single addon item")
	}
	o.Names = args
	if o.dynamic == nil {
		var err error
		if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
			return err
		}
	}

	// setup _KUBE_SERVER_INFO
	if viper.Get(constant.CfgKeyServerInfo) == nil {
		cfg, _ := o.Factory.ToRESTConfig()
		cli, err := discoverycli.NewDiscoveryClientForConfig(cfg)
		if err != nil {
			return err
		}
		ver, err := cli.ServerVersion()
		if err != nil {
			return err
		}
		viper.SetDefault(constant.CfgKeyServerInfo, *ver)
	}

	return nil
}

func (o *addonCmdOpts) fetchAddonObj() error {
	ctx := context.TODO()
	obj, err := o.dynamic.Resource(o.GVR).Get(ctx, o.Names[0], metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &o.addon); err != nil {
		return err
	}
	return nil
}

func addonDescribeHandler(o *addonCmdOpts, cmd *cobra.Command, args []string) error {
	printRow := func(tbl *printer.TablePrinter, name string, item *extensionsv1alpha1.AddonInstallSpecItem) {
		pvEnabled := ""
		replicas := ""

		if item.PVEnabled != nil {
			pvEnabled = fmt.Sprintf("%v", *item.PVEnabled)
		}
		if item.Replicas != nil {
			replicas = fmt.Sprintf("%d", *item.Replicas)
		}

		printQuantity := func(q resource.Quantity, ok bool) string {
			if ok {
				return q.String()
			}
			return ""
		}

		q, ok := item.Resources.Requests[corev1.ResourceStorage]
		storageVal := printQuantity(q, ok)

		q, ok = item.Resources.Requests[corev1.ResourceCPU]
		cpuVal := printQuantity(q, ok)
		q, ok = item.Resources.Limits[corev1.ResourceCPU]
		cpuVal = fmt.Sprintf("%s/%s", cpuVal, printQuantity(q, ok))

		q, ok = item.Resources.Requests[corev1.ResourceMemory]
		memVal := printQuantity(q, ok)
		q, ok = item.Resources.Limits[corev1.ResourceMemory]
		memVal = fmt.Sprintf("%s/%s", memVal, printQuantity(q, ok))

		tbl.AddRow(name,
			replicas,
			storageVal,
			cpuVal,
			memVal,
			item.StorageClass,
			item.Tolerations,
			pvEnabled,
		)
	}
	printInstalled := func(tbl *printer.TablePrinter) error {
		installSpec := o.addon.Spec.InstallSpec
		printRow(tbl, "main", &installSpec.AddonInstallSpecItem)
		for _, e := range installSpec.ExtraItems {
			printRow(tbl, e.Name, &e.AddonInstallSpecItem)
		}
		return nil
	}

	labels := []string{}
	for k, v := range o.addon.Labels {
		if strings.Contains(k, constant.APIGroup) {
			labels = append(labels, fmt.Sprintf("%s=%s", k, v))
		}
	}
	printer.PrintPairStringToLine("Name", o.addon.Name, 0)
	printer.PrintPairStringToLine("Description", o.addon.Spec.Description, 0)
	printer.PrintPairStringToLine("Labels", strings.Join(labels, ","), 0)
	printer.PrintPairStringToLine("Type", string(o.addon.Spec.Type), 0)
	printer.PrintPairStringToLine("Extras", strings.Join(o.addon.GetExtraNames(), ","), 0)
	printer.PrintPairStringToLine("Status", string(o.addon.Status.Phase), 0)
	var autoInstall bool
	if o.addon.Spec.Installable != nil {
		autoInstall = o.addon.Spec.Installable.AutoInstall
	}
	printer.PrintPairStringToLine("Auto-install", strconv.FormatBool(autoInstall), 0)
	printer.PrintPairStringToLine("Installable", strings.Join(o.addon.Spec.Installable.GetSelectorsStrings(), ","), 0)

	switch o.addon.Status.Phase {
	case extensionsv1alpha1.AddonEnabled:
		printer.PrintTitle("Installed Info")
		printer.PrintLineWithTabSeparator()
		if err := printer.PrintTable(o.Out, nil, printInstalled,
			"NAME", "REPLICAS", "STORAGE", "CPU (REQ/LIMIT)", "MEMORY (REQ/LIMIT)", "STORAGE-CLASS",
			"TOLERATIONS", "PV Enabled"); err != nil {
			return err
		}
	default:
		printer.PrintLineWithTabSeparator()
		for _, di := range o.addon.Spec.GetSortedDefaultInstallValues() {
			printInstallable := func(tbl *printer.TablePrinter) error {
				if len(di.Selectors) == 0 {
					printer.PrintLineWithTabSeparator(
						printer.NewPair("Default install selector", "NONE"),
					)
				} else {
					printer.PrintLineWithTabSeparator(
						printer.NewPair("Default install selector", strings.Join(di.GetSelectorsStrings(), ",")),
					)
				}
				installSpec := di.AddonInstallSpec
				printRow(tbl, "main", &installSpec.AddonInstallSpecItem)
				for _, e := range installSpec.ExtraItems {
					printRow(tbl, e.Name, &e.AddonInstallSpecItem)
				}
				return nil
			}
			if err := printer.PrintTable(o.Out, nil, printInstallable,
				"NAME", "REPLICAS", "STORAGE", "CPU (REQ/LIMIT)", "MEMORY (REQ/LIMIT)", "STORAGE-CLASS",
				"TOLERATIONS", "PV Enabled"); err != nil {
				return err
			}
			printer.PrintLineWithTabSeparator()
		}
	}
	return nil
}

func addonEnableDisableHandler(o *addonCmdOpts, cmd *cobra.Command, args []string) error {
	// record the flags that been set by user
	var flags []*pflag.Flag
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		flags = append(flags, flag)
	})
	return o.buildPatch(flags)
}

func (o *addonCmdOpts) buildEnablePatch(flags []*pflag.Flag, spec, install map[string]interface{}) (err error) {
	var installSpec extensionsv1alpha1.AddonInstallSpec
	// only using named return value in defer function
	defer func() {
		var b []byte
		b, err = json.Marshal(&installSpec)
		if err != nil {
			return
		}
		if err = json.Unmarshal(b, &install); err != nil {
			return
		}
	}()

	if o.addonEnableFlags.useDefault() {
		if len(o.addon.Spec.DefaultInstallValues) == 0 {
			installSpec.Enabled = true
			return nil
		}

		for _, di := range o.addon.Spec.GetSortedDefaultInstallValues() {
			if len(di.Selectors) == 0 {
				installSpec = di.AddonInstallSpec
				break
			}
			for _, s := range di.Selectors {
				if !s.MatchesFromConfig() {
					continue
				}
				installSpec = di.AddonInstallSpec
				break
			}
		}
		installSpec.Enabled = true
		return nil
	}

	getExtraItemIndex := func(name string) int {
		var pItem *extensionsv1alpha1.AddonInstallExtraItem
		for i, eItem := range installSpec.ExtraItems {
			if eItem.Name == name {
				return i
			}
		}
		pItem = &extensionsv1alpha1.AddonInstallExtraItem{
			Name: name,
		}
		installSpec.ExtraItems = append(installSpec.ExtraItems, *pItem)
		return len(installSpec.ExtraItems) - 1
	}

	twoTuplesProcessor := func(s, flag string,
		valueTransformer func(s, flag string) (interface{}, error),
		valueAssigner func(*extensionsv1alpha1.AddonInstallSpecItem, interface{}),
	) error {
		t := strings.SplitN(s, ":", 2)
		l := len(t)
		var name string
		var result interface{}
		var err error
		switch l {
		case 2:
			name = t[0]
			fallthrough
		case 1:
			if valueTransformer != nil {
				result, err = valueTransformer(t[l-1], flag)
				if err != nil {
					return err
				}
			} else {
				result = t[l-1]
			}
		default:
			return fmt.Errorf("wrong flag value --%s=%s", flag, s)
		}
		if name == "" {
			valueAssigner(&installSpec.AddonInstallSpecItem, result)
		} else {
			idx := getExtraItemIndex(name)
			valueAssigner(&installSpec.ExtraItems[idx].AddonInstallSpecItem, result)
		}
		return nil
	}

	reqLimitResTransformer := func(s, flag string) (interface{}, error) {
		t := strings.SplitN(s, "/", 2)
		if len(t) != 2 {
			return nil, fmt.Errorf("wrong flag value --%s=%s", flag, s)
		}
		reqLim := [2]resource.Quantity{}
		processTuple := func(i int) error {
			if t[i] == "" {
				return nil
			}
			q, err := resource.ParseQuantity(t[i])
			if err != nil {
				return err
			}
			reqLim[i] = q
			return nil
		}
		for i := range t {
			if err := processTuple(i); err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
		}
		return reqLim, nil
	}

	f := o.addonEnableFlags
	for _, v := range f.ReplicaCountSets {
		if err := twoTuplesProcessor(v, "replicas", func(s, flag string) (interface{}, error) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
			r := int32(v)
			return &r, nil
		}, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Replicas = i.(*int32)
		}); err != nil {
			return err
		}
	}

	for _, v := range f.StorageClassSets {
		if err := twoTuplesProcessor(v, "storage-class", nil, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.StorageClass = i.(string)
		}); err != nil {
			return err
		}
	}

	for _, v := range f.TolerationsSet {
		if err := twoTuplesProcessor(v, "tolerations", nil, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Tolerations = i.(string)
		}); err != nil {
			return err
		}
	}

	for _, v := range f.StorageSets {
		if err := twoTuplesProcessor(v, "storage", func(s, flag string) (interface{}, error) {
			q, err := resource.ParseQuantity(s)
			if err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
			return q, nil
		}, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Resources.Requests[corev1.ResourceStorage] = i.(resource.Quantity)
		}); err != nil {
			return err
		}
	}

	for _, v := range f.CPUSets {
		if err := twoTuplesProcessor(v, "cpu", reqLimitResTransformer, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			reqLim := i.([2]resource.Quantity)
			item.Resources.Requests[corev1.ResourceCPU] = reqLim[0]
			item.Resources.Limits[corev1.ResourceCPU] = reqLim[1]
		}); err != nil {
			return err
		}
	}

	for _, v := range f.MemorySets {
		if err := twoTuplesProcessor(v, "memory", reqLimitResTransformer, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			reqLim := i.([2]resource.Quantity)
			item.Resources.Requests[corev1.ResourceMemory] = reqLim[0]
			item.Resources.Limits[corev1.ResourceMemory] = reqLim[1]
		}); err != nil {
			return err
		}
	}
	return nil
}

func (o *addonCmdOpts) buildPatch(flags []*pflag.Flag) error {
	var err error
	spec := map[string]interface{}{}
	status := map[string]interface{}{}
	install := map[string]interface{}{}

	if o.addonEnableFlags != nil {
		if o.addon.Status.Phase == extensionsv1alpha1.AddonFailed {
			status["phase"] = nil
		}
		if err = o.buildEnablePatch(flags, spec, install); err != nil {
			return err
		}
	} else {
		if !o.addon.Spec.InstallSpec.GetEnabled() {
			fmt.Fprintf(o.Out, "%s/%s is already disabled\n", o.GVR.GroupResource().String(), o.Names[0])
			return cmdutil.ErrExit
		}
		install["enabled"] = false
	}

	if err = unstructured.SetNestedField(spec, install, "install"); err != nil {
		return err
	}

	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"spec": spec,
		},
	}
	if len(status) > 0 {
		phase := ""
		if p, ok := status["phase"]; ok && p != nil {
			phase = p.(string)
		}
		fmt.Printf("patching addon 'status.phase=%s' to 'status.phase=%v' will result addon install spec (spec.install) not being updated\n",
			o.addon.Status.Phase, phase)
		obj.Object["status"] = status
		o.Subresource = "status"
	}
	bytes, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	o.Patch = string(bytes)
	return nil
}

func addonListRun(o *list.ListOptions) error {
	// if format is JSON or YAML, use default printer to output the result.
	if o.Format == printer.JSON || o.Format == printer.YAML {
		_, err := o.Run()
		return err
	}

	// get and output the result
	o.Print = false
	r, err := o.Run()
	if err != nil {
		return err
	}

	infos, err := r.Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		fmt.Fprintln(o.IOStreams.Out, "No addon found")
		return nil
	}

	printRows := func(tbl *printer.TablePrinter) error {
		for _, info := range infos {
			addon := &extensionsv1alpha1.Addon{}
			obj := info.Object.(*unstructured.Unstructured)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, addon); err != nil {
				return err
			}
			extraNames := addon.GetExtraNames()
			var selectors []string
			var autoInstall bool
			if addon.Spec.Installable != nil {
				selectors = addon.Spec.Installable.GetSelectorsStrings()
				autoInstall = addon.Spec.Installable.AutoInstall
			}
			tbl.AddRow(addon.Name,
				addon.Spec.Type,
				addon.Status.Phase,
				strings.Join(extraNames, ","),
				autoInstall,
				strings.Join(selectors, ";"),
			)
		}
		return nil
	}

	if err = printer.PrintTable(o.Out, nil, printRows,
		"NAME", "TYPE", "STATUS", "EXTRAS", "AUTO-INSTALL", "INSTALLABLE-SELECTOR"); err != nil {
		return err
	}
	return nil
}
