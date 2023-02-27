package addon

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type addonCmdOpts struct {
	Factory cmdutil.Factory
	genericclioptions.IOStreams

	MemorySets       []string
	CPUSets          []string
	StorageSets      []string
	ReplicaCountSets []string
	StorageClassSets []string
	TolerationsSet   []string
}

// NewAddonCmd for addon functions
func NewAddonCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "addon",
		Short: "Addon command",
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
		Short:             "List addons",
		Aliases:           []string{"ls"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(addonListRun(o))
		},
	}
	return cmd
}

func newDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addonCmdOpts{
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
		Factory:   f,
		IOStreams: streams,
	}

	//# kbcli addon enable flags:
	//# [--memory [extraName:]<request>:<limit> (can specify multiple if has extra items)]
	//# [--cpu [extraName:]<request>:<limit> (can specify multiple if has extra items)]
	//# [--storage [extraName:]<request> (can specify multiple if has extra items)]
	//# [--replicas [extraName:]<N> (can specify multiple if has extra items)]
	//# [--storage-class [extraName:]<SC name> (can specify multiple if has extra items)]
	//# [--tolerations [extraName:]<toleration JSON list items> (can specify multiple if has extra items)]
	//# [--dry-run] # TODO

	cmd := &cobra.Command{
		Use:   "enable ADDON_NAME",
		Short: "Enable an addon",
		Example: templates.Examples(`
    	# Enabled "prometheus" addon
    	kbcli addon enable prometheus
    
        # Enabled "prometheus" addon with custom resources settings
    	kbcli addon enable prometheus --memory 512Mi:4Gi --storage 8Gi --replicas 2
    
        # Enabled "prometheus" addon and its extra alertmanager component with custom resources settings 
    	kbcli addon enable prometheus --memory 512Mi:4Gi --storage 8Gi --replicas 2 \
  			--memory alertmanager:16Mi:256Mi --storage: alertmanager:1Gi --replicas alertmanager:2 

        # Enabled "prometheus" addon with tolerations 
    	kbcli addon enable prometheus --tolerations '[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]' \
			--tolerations 'alertmanager:[[{"key":"taintkey","operator":"Equal","effect":"NoSchedule","value":"true"}]]'`),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
		},
	}
	cmd.Flags().StringArrayVar(&o.MemorySets, "--memory", []string{},
		"Sets addon memory resource values (--memory [extraName:]<request>:<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.CPUSets, "--cpu", []string{},
		"Sets addon CPU resource values (--cpu [extraName:]<request>:<limit>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.StorageSets, "--storage", []string{},
		"Sets addon storage size (--storage [extraName:]<request>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.ReplicaCountSets, "--replicas", []string{},
		"Sets addon component replica count (--replicas [extraName:]<N>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.StorageClassSets, "--storage-class", []string{},
		"Sets addon storage class name (--storage-class [extraName:]<SC name>) (can specify multiple if has extra items))")
	cmd.Flags().StringArrayVar(&o.TolerationsSet, "--tolerations", []string{},
		"Sets addon pod tolerations (--tolerations [extraName:]<toleration JSON list items>) (can specify multiple if has extra items))")
	return cmd
}

func newDisableCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &addonCmdOpts{
		Factory:   f,
		IOStreams: streams,
	}
	cmd := &cobra.Command{
		Use:   "disable ADDON_NAME",
		Short: "Disable an addon",
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(cmd, args))
		},
	}
	return cmd
}

// Complete receive exec parameters
func (o *addonCmdOpts) complete(cmd *cobra.Command, args []string) error {
	var err error
	return err
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

	// NAME                 TYPE  STATUS    EXTRAS        INSTALLABLE-SELECTOR         AUTO-INSTALL
	if err = printer.PrintTable(o.Out, nil, printRows, "NAME", "TYPE", "STATUS", "EXTRAS", "INSTALLABLE-SELECTOR", "AUTO-INSTALL"); err != nil {
		return err
	}
	return nil
}
