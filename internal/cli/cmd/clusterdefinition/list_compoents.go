package clusterdefinition

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listComponentsExample = templates.Examples(`
		# List all components belong to the cluster definition.
		kbcli clusterdefinition list-components apecloud-mysql`)

	componentsTableHeader = []interface{}{
		"Name",
		"WorkloadType",
		"CharacterType",
	}
)

func NewListComponentsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterDefGVR())
	cmd := &cobra.Command{
		Use:               "list-components",
		Short:             "List cluster definition components.",
		Example:           listComponentsExample,
		Aliases:           []string{"ls-comps"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(validate(args))
			o.FieldSelector = fmt.Sprintf("metadata.name=%s", args[0])
			util.CheckErr(run(o))
		},
	}
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespace", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	return cmd
}

func validate(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("you must specify the clusterdefinition name to list")
	} else if len(args) > 1 {
		return fmt.Errorf("too many clusterdefinition names you have input")
	}
	return nil
}

func run(o *list.ListOptions) error {
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
		return fmt.Errorf("no clusterdefinition %s found", o.Names[0])
	}
	p := printer.NewTablePrinter(o.Out)
	p.SetHeader(componentsTableHeader...)
	for _, info := range infos {
		var cd v1alpha1.ClusterDefinition
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cd)
		if err != nil {
			return err
		}
		for _, comp := range cd.Spec.ComponentDefs {
			row := make([]interface{}, len(componentsTableHeader))
			row[0] = comp.Name
			row[1] = comp.WorkloadType
			row[2] = comp.CharacterType
			p.AddRow(row...)
		}
	}
	p.Print()
	return nil
}
