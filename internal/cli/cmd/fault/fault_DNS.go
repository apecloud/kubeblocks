package fault

import (
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

// TODO: add more examples
var faultDNSExample = `
	
`

type DNSChaosOptions struct {
	Patterns []string `json:"patterns"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *DNSChaosOptions) createInputs(f cmdutil.Factory, use string, short string, buildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultDNSExample,
		CueTemplateName: CueTemplateDNSChaos,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceDNSChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewDNSChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "DNS",
		Short: "DNS chaos.",
	}
	cmd.AddCommand(
		NewRandomCmd(f, streams),
		NewErrorCmd(f, streams),
	)
	return cmd
}

func NewRandomCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &DNSChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.RandomAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Random,
		RandomShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewErrorCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &DNSChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.ErrorAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Error,
		ErrorShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *DNSChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringArrayVar(&o.Patterns, "patterns", nil, `Select the domain name template that matches the failure behavior, and support placeholders ? and wildcards *.`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

// Validate TODO
func (o *DNSChaosOptions) Validate() error {
	err := o.BaseValidate()
	if err != nil {
		return err
	}
	return nil
}

// Complete TODO
func (o *DNSChaosOptions) Complete() error {
	err := o.BaseComplete()
	if err != nil {
		return err
	}
	return nil
}

func (o *DNSChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.DNSChaos{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, c); err != nil {
		return err
	}

	data, e := runtime.DefaultUnstructuredConverter.ToUnstructured(c)
	if e != nil {
		return e
	}
	obj.SetUnstructuredContent(data)
	return nil
}
