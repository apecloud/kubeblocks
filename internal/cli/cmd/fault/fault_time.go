package fault

import (
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var faultTimeExample = templates.Examples(`
	#Shifts the clock back five seconds.
	kbcli fault time --timeOffset=-5s
`)

type StressTimeOptions struct {
	TimeOffset string `json:"timeOffset"`

	ClockIds []string `json:"clockIds,omitempty"`

	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *StressTimeOptions) createInputs(f cmdutil.Factory, use string, short string, buildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultTimeExample,
		CueTemplateName: CueTemplateTimeChaos,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceTimeChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewTimeChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &StressTimeOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		util.CheckErr(cmd.MarkFlagRequired("timeOffset"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Time,
		TimeShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *StressTimeOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.TimeOffset, "timeOffset", "", "Specifies the length of the time offset. For example: -5s, -10m100ns.")
	cmd.Flags().StringArrayVar(&o.ClockIds, "clockIds", nil, `Specifies the clock on which the time offset acts.If it's empty, it will be set to ['CLOCK_REALTIME'].See clock_gettime [https://man7.org/linux/man-pages/man2/clock_gettime.2.html] document for details.`)
	cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, `Specifies the injected container name. For example: mysql. If it's empty, the first container will be injected.`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *StressTimeOptions) Validate() error {
	return o.BaseValidate()
}

func (o *StressTimeOptions) Complete() error {
	return o.BaseComplete()
}

func (o *StressTimeOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.TimeChaos{}
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
