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
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

// TODO: add more examples
var faultIOExample = `
	
`

type IOChaosOptions struct {
	Delay string `json:"delay"`
	Errno int    `json:"errno"`

	// Atrr           v1alpha1.AttrOverrideSpec `json:"attr_override_spec,omitempty"`
	VolumePath string `json:"volumePath"`
	Path       string `json:"path"`
	Percent    int    `json:"percent"`

	ContainerNames []string `json:"containerNames,omitempty"`
	Methods        []string `json:"methods,omitempty"`

	FaultBaseOptions

	create.BaseOptions
}

func NewIOChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "IO",
		Short: "IO chaos.",
	}
	cmd.AddCommand(
		NewIOLatencyCmd(f, streams),
		NewIOFaultCmd(f, streams),
		NewIOAttrOverrideCmd(f, streams),
	)
	return cmd
}

func (o *IOChaosOptions) createInputs(f cmdutil.Factory, use string, short string, buildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultIOExample,
		CueTemplateName: CueTemplateIOChaos,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceIOChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewIOLatencyCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &IOChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.IoLatency)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().StringVar(&o.Delay, "delay", "", `specific delay time.`)

		util.CheckErr(cmd.MarkFlagRequired("delay"))
		util.CheckErr(cmd.MarkFlagRequired("volume-path"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Latency,
		LatencyShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewIOFaultCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &IOChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.IoFaults)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().IntVar(&o.Errno, "errno", 0, `the returned error number.`)

		util.CheckErr(cmd.MarkFlagRequired("errno"))
		util.CheckErr(cmd.MarkFlagRequired("volume-path"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Fault,
		FaultShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

// NewIOAttrOverrideCmd TODO
func NewIOAttrOverrideCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &IOChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.IoAttrOverride)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		AttrOverride,
		AttrOverrideShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *IOChaosOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.VolumePath, "volume-path", "", `The mount point of the volume in the target container must be the root directory of the mount.`)
	cmd.Flags().StringVar(&o.Path, "path", "", `The effective scope of the injection error can be a wildcard or a single file.`)
	cmd.Flags().IntVar(&o.Percent, "percent", 0, `Probability of failure per operation, in %.`)
	cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, "the name of the container, such as mysql, prometheus.If it's empty, the first container will be injected.")
	cmd.Flags().StringArrayVar(&o.Methods, "methods", nil, "Types of file system calls that need to inject faults.")

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

// Validate TODO
func (o *IOChaosOptions) Validate() error {
	err := o.BaseValidate()
	if err != nil {
		return err
	}
	return nil
}

// Complete TODO
func (o *IOChaosOptions) Complete() error {
	err := o.BaseComplete()
	if err != nil {
		return err
	}
	return nil
}

func (o *IOChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.IOChaos{}
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
