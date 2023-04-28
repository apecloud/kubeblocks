package fault

import (
	"fmt"
	"strconv"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
)

type NetworkChaosOptions struct {
	// Specify the network direction
	Direction string `json:"direction"`
	// Indicates a network target outside of Kubernetes, which can be an IPv4 address or a domain name,
	// such as "www.baidu.com". Only works with direction: to.
	ExternalTargets []string `json:"externalTargets,omitempty"`
	// Specifies the labels that target Pods come with.
	TargetLabel map[string]string `json:"targetLabel,omitempty"`
	// Specifies the namespaces to which target Pods belong.
	TargetNamespaceSelector string `json:"targetNamespaceSelector"`

	TargetMode  string `json:"targetMode"`
	TargetValue string `json:"targetValue"`

	// The percentage of packet loss
	Loss string `json:"loss,omitempty"`
	// The percentage of packet corruption
	Corrupt string `json:"corrupt,omitempty"`
	// The percentage of packet duplication
	Duplicate string `json:"duplicate,omitempty"`
	// The latency of delay
	Latency string `json:"latency,omitempty"`
	// The jitter of delay
	Jitter string `json:"jitter"`

	// The correlation of loss or corruption or duplication or delay
	Correlation string `json:"correlation"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *NetworkChaosOptions) createInputs(f cmdutil.Factory, buildFlags func(*cobra.Command), use string, short string, cueTemplateName string) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultPodExample,
		CueTemplateName: cueTemplateName,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceNetworkChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewNetworkChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "network attack.",
	}
	cmd.AddCommand(
		NewPartitionCmd(f, streams),
		NewLossCmd(f, streams),
		NewDelayCmd(f, streams),
		NewDuplicateCmd(f, streams),
		NewCorruptCmd(f, streams),
		NewBandwidthCmd(f, streams),
	)
	return cmd
}

func NewPartitionCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PartitionAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Partition,
		PartitionShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func NewLossCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.LossAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().StringVar(&o.Loss, "loss", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
		cmd.Flags().StringVar(&o.Correlation, "correlation", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Loss,
		LossShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func NewDelayCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.DelayAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().StringVar(&o.Latency, "latency", "", `the length of time to delay.`)
		cmd.Flags().StringVar(&o.Jitter, "jitter", "0ms", `the variation range of the delay time.`)
		cmd.Flags().StringVar(&o.Correlation, "correlation", "0", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Delay,
		DelayShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func NewDuplicateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.DuplicateAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().StringVar(&o.Duplicate, "duplicate", "", `the probability of a packet being repeated. Value range: [0, 100].`)
		cmd.Flags().StringVar(&o.Correlation, "correlation", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Duplicate,
		DuplicateShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func NewCorruptCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.CorruptAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().StringVar(&o.Corrupt, "corrupt", "", `Indicates the probability of a packet error occurring. Value range: [0, 100].`)
		cmd.Flags().StringVar(&o.Correlation, "correlation", "0", `Indicates the correlation between the probability of a packet error occurring and whether it occurred the previous time. Value range: [0, 100].`)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Corrupt,
		CorruptShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func NewBandwidthCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.BandwidthAction)},
	}
	// TODO
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
	}
	inputs := o.createInputs(
		f,
		BuildFlags,
		Bandwidth,
		BandwidthShort,
		CueTemplateNetworkChaos,
	)

	return create.BuildCommand(*inputs)
}

func (o *NetworkChaosOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.Direction, "direction", "to", `You can select "to"" or "from"" or "both"".`)
	cmd.Flags().StringArrayVar(&o.ExternalTargets, "external-targets", nil, "a network target outside of Kubernetes, which can be an IPv4 address or a domain name,\n\t such as \"www.baidu.com\". Only works with direction: to.")
	cmd.Flags().StringVar(&o.TargetMode, "target-mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.TargetValue, "target-value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringToStringVar(&o.TargetLabel, "target-label", nil, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringVar(&o.TargetNamespaceSelector, "target-namespace-selector", "", `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *NetworkChaosOptions) Validate() error {
	if o.TargetValue == "" && (o.TargetMode == "fixed" || o.TargetMode == "fixed-percent" || o.TargetMode == "random-max-percent") {
		return fmt.Errorf("you must use --value to specify an integer")
	}

	if _, err := strconv.Atoi(o.TargetValue); o.TargetValue != "" && err != nil {
		return fmt.Errorf("invalid value:%s; must be an integer", o.TargetValue)
	}
	err := o.BaseValidate()
	if err != nil {
		return err
	}
	return nil
}

func (o *NetworkChaosOptions) Complete() error {
	err := o.BaseComplete()
	if err != nil {
		return err
	}
	return nil
}

func (o *NetworkChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.NetworkChaos{}
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
