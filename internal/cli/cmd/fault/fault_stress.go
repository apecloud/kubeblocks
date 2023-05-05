package fault

import (
	"fmt"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

var faultStressExample = templates.Examples(`
	# Creating a process in a container consumes CPU and memory resources continuously, making the CPU load up to 50%, and the memory up to 100MB.
	kbcli fault stress --cpu-workers=2 --cpu-load=50 --memory-workers=1 --memory-size=100Mi
`)

type StressChaosOptions struct {
	CPUWorkers int `json:"cpuWorkers"`

	CPULoad int `json:"cpuLoad"`

	MemoryWorkers int `json:"memoryWorkers"`

	MemorySize string `json:"memorySize"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *StressChaosOptions) createInputs(f cmdutil.Factory, use string, short string, buildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultStressExample,
		CueTemplateName: CueTemplateStressChaos,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceStressChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewStressChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &StressChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Stress,
		StressShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *StressChaosOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().IntVar(&o.CPUWorkers, "cpu-workers", 0, `Specifies the number of threads that exert CPU pressure.`)
	cmd.Flags().IntVar(&o.CPULoad, "cpu-load", 0, `Specifies the percentage of CPU occupied. 0 means no extra load added, 100 means full load. The total load is workers * load.`)
	cmd.Flags().IntVar(&o.MemoryWorkers, "memory-workers", 0, `Specifies the number of threads that apply memory pressure.`)
	cmd.Flags().StringVar(&o.MemorySize, "memory-size", "", `Specify the size of the allocated memory or the percentage of the total memory, and the sum of the allocated memory is size. For example:256MB or 25%`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *StressChaosOptions) Validate() error {
	if o.MemoryWorkers == 0 && o.CPUWorkers == 0 {
		return fmt.Errorf("the CPU or Memory workers must have at least one greater than 0, Use --cpu-workers or --memory-workers to specify")
	}

	return o.BaseValidate()
}

func (o *StressChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *StressChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.StressChaos{}
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
