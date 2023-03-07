package addon

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type addonDescribeCmdOpts struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams
}

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
	*patch.Options
	genericclioptions.IOStreams

	Factory cmdutil.Factory
	dynamic dynamic.Interface
	addon   extensionsv1alpha1.Addon

	addonEnableFlags *addonEnableFlags
}

// NewAddonCmd for addon functions
func NewAddonCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "Addon command",
	}
	cmd.AddCommand(
		newListCmd(f, streams),
		// newDescribeCmd(f, streams),
		newEnableCmd(f, streams),
		newDisableCmd(f, streams),
	)
	return cmd
}

func newListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.AddonGVR())
	cmd := &cobra.Command{
		Use:               "list ",
		Short:             "List addons",
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
	o := &addonDescribeCmdOpts{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "describe ADDON_NAME",
		Short: "Describe an addon specification",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
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
		Use:   "enable ADDON_NAME",
		Short: "Enable an addon",
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
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.MemorySets, "memory", []string{},
		"Sets addon memory resource values (--memory [extraName:]<request>:<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.CPUSets, "cpu", []string{},
		"Sets addon CPU resource values (--cpu [extraName:]<request>:<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.StorageSets, "storage", []string{},
		"Sets addon storage size (--storage [extraName:]<request>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.ReplicaCountSets, "replicas", []string{},
		"Sets addon component replica count (--replicas [extraName:]<N>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.addonEnableFlags.StorageClassSets, "storage-class", []string{},
		"Sets addon storage class name (--storage-class [extraName:]<SC name>) (can specify multiple if has extra items))")
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
	}
	cmd := &cobra.Command{
		Use:   "disable ADDON_NAME",
		Short: "Disable an addon",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.Options.AddFlags(cmd)
	return cmd
}

// Complete receive exec parameters
func (o *addonDescribeCmdOpts) complete(cmd *cobra.Command, args []string) error {
	var err error
	return err
}

func (o *addonCmdOpts) complete(cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 0 {
		return fmt.Errorf("missing addon name")
	}
	if len(args) > 1 {
		return fmt.Errorf("only accept enable/disable single addon item")
	}
	o.Names = args

	// record the flags that been set by user
	var flags []*pflag.Flag
	cmd.Flags().Visit(func(flag *pflag.Flag) {
		flags = append(flags, flag)
	})

	if o.dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}

	ctx := context.TODO()
	obj, err := o.dynamic.Resource(o.GVR).Get(ctx, o.Names[0], metav1.GetOptions{})
	if err != nil {
		return err
	}
	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &o.addon); err != nil {
		return err
	}
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

	getExtraItem := func(name string) *extensionsv1alpha1.AddonInstallExtraItem {
		var pItem *extensionsv1alpha1.AddonInstallExtraItem
		for i, eItem := range installSpec.ExtraItems {
			if eItem.Name == name {
				pItem = &installSpec.ExtraItems[i]
				break
			}
		}
		if pItem == nil {
			pItem = &extensionsv1alpha1.AddonInstallExtraItem{
				Name: name,
			}
			installSpec.ExtraItems = append(installSpec.ExtraItems, *pItem)
		}
		return pItem
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
			pItem := getExtraItem(name)
			valueAssigner(&pItem.AddonInstallSpecItem, result)
		}
		return nil
	}

	reqLimitResTransformer := func(s, flag string) (interface{}, error) {
		t := strings.SplitN(s, "/", 2)
		if len(t) != 2 {
			return nil, fmt.Errorf("wrong flag value --%s=%s", flag, s)
		}
		reqLim := [2]resource.Quantity{}
		proccessTuple := func(i int) error {
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
			if err := proccessTuple(i); err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
		}
		return reqLim, nil
	}

	f := o.addonEnableFlags
	for _, v := range f.ReplicaCountSets {
		twoTuplesProcessor(v, "replicas", func(s, flag string) (interface{}, error) {
			v, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
			r := int32(v)
			return &r, nil
		}, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Replicas = i.(*int32)
		})
	}

	for _, v := range f.StorageClassSets {
		twoTuplesProcessor(v, "storage-class", nil, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.StorageClass = i.(string)
		})
	}

	for _, v := range f.TolerationsSet {
		twoTuplesProcessor(v, "tolerations", nil, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Tolerations = i.(string)
		})
	}

	for _, v := range f.StorageSets {
		twoTuplesProcessor(v, "storage", func(s, flag string) (interface{}, error) {
			q, err := resource.ParseQuantity(s)
			if err != nil {
				return nil, fmt.Errorf("wrong flag value --%s=%s, with error %v", flag, s, err)
			}
			return q, nil
		}, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			item.Resources.Requests[corev1.ResourceStorage] = i.(resource.Quantity)
		})
	}

	for _, v := range f.CPUSets {
		twoTuplesProcessor(v, "cpu", reqLimitResTransformer, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			reqLim := i.([2]resource.Quantity)
			item.Resources.Requests[corev1.ResourceCPU] = reqLim[0]
			item.Resources.Limits[corev1.ResourceCPU] = reqLim[1]
		})
	}

	for _, v := range f.MemorySets {
		twoTuplesProcessor(v, "memory", reqLimitResTransformer, func(item *extensionsv1alpha1.AddonInstallSpecItem, i interface{}) {
			reqLim := i.([2]resource.Quantity)
			item.Resources.Requests[corev1.ResourceMemory] = reqLim[0]
			item.Resources.Limits[corev1.ResourceMemory] = reqLim[1]
		})
	}

	return nil
}

func (o *addonCmdOpts) buildPatch(flags []*pflag.Flag) error {
	var err error
	spec := map[string]interface{}{}
	install := map[string]interface{}{}

	if o.addonEnableFlags != nil {
		if o.addon.Spec.InstallSpec.Enabled {
			fmt.Fprintf(o.Out, "%s/%s is already enbled\n", o.GVR.GroupResource().String(), o.Names[0])
			return cmdutil.ErrExit
		}
		if err = o.buildEnablePatch(flags, spec, install); err != nil {
			return err
		}
	} else {
		if !o.addon.Spec.InstallSpec.Enabled {
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
				strings.Join(selectors, ";"),
				autoInstall,
			)
		}
		return nil
	}

	if err = printer.PrintTable(o.Out, nil, printRows,
		"NAME", "TYPE", "STATUS", "EXTRAS", "INSTALLABLE-SELECTOR", "AUTO-INSTALL"); err != nil {
		return err
	}
	return nil
}
