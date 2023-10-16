package clusterdefinition

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/list"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	listServiceRefExample = templates.Examples(`
		# List cluster references name declared in a cluster definition.
		kbcli clusterdefinition list-service-reference apecloud-mysql`)
)

func NewListServiceReferenceCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := list.NewListOptions(f, streams, types.ClusterDefGVR())
	o.AllNamespaces = true
	cmd := &cobra.Command{
		Use:               "list-service-reference",
		Short:             "List cluster references declared in a cluster definition.",
		Example:           listServiceRefExample,
		Aliases:           []string{"ls-sr"},
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, o.GVR),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(validate(args))
			o.Names = args
			util.CheckErr(listServiceRef(o))
		},
	}
	return cmd
}

func listServiceRef(o *list.ListOptions) error {
	o.Print = false

	r, err := o.Run()
	if err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}

	p := printer.NewTablePrinter(o.Out)
	p.SetHeader("NAME", "COMPONENT", "SERVICE-KIND", "SERVICE-VERSION")
	p.SortBy(4, 1)
	for _, info := range infos {
		var cd v1alpha1.ClusterDefinition
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(info.Object.(*unstructured.Unstructured).Object, &cd); err != nil {
			return err
		}
		for _, comp := range cd.Spec.ComponentDefs {
			if comp.ServiceRefDeclarations == nil {
				continue
			}
			for _, serviceDec := range comp.ServiceRefDeclarations {
				for _, ref := range serviceDec.ServiceRefDeclarationSpecs {
					p.AddRow(serviceDec.Name, comp.Name, ref.ServiceKind, ref.ServiceVersion)
				}
			}
		}
	}

	if p.Tbl.Length() == 0 {
		fmt.Printf("No service references are declared in cluster definition %s", o.Names)
	} else {
		p.Print()
	}
	return nil
}
