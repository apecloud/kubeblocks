package fault

import (
	"fmt"

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

// TODO: add more examples
var faultNetWorkExample = templates.Examples(`
	# Isolate all pods network under the default namespace from the outside world, including the k8s internal network.
	kbcli fault network partition

	# The specified pod is isolated from the k8s external network "kubeblocks.io".
	kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --external-targets=kubeblocks.io
	
	# Isolate the network between two pods.
	kbcli fault network partition --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-1 --target-label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2
	
	// Like the partition command, the target can be specified through --target-label or --external-targets. The pod only has obstacles in communicating with this target. If the target is not specified, all communication will be blocked.
	# Block all pod communication under the default namespace, resulting in a 50% packet loss rate.
	kbcli fault network loss --loss=50
	
	# Block the specified pod communication, so that the packet loss rate is 50%.
	kbcli fault network loss --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --loss=50
	
	kbcli fault network corrupt --corrupt=50

	# Blocks specified pod communication with a 50% packet corruption rate.
	kbcli fault network corrupt --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --corrupt=50
	
	kbcli fault network duplicate --duplicate=50

	# Block specified pod communication so that the packet repetition rate is 50%.
	kbcli fault network duplicate --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --duplicate=50
	
	kbcli fault network delay --latency=10s

	# Block the communication of the specified pod, causing its network delay for 10s.
	kbcli fault network delay --label=statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2 --latency=10s
`)

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

	Rate string `json:"rate,omitempty"`

	Limit uint32 `json:"limit"`

	Buffer uint32 `json:"buffer"`

	Peakrate uint64 `json:"peakrate"`

	Minburst uint32 `json:"minburst"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *NetworkChaosOptions) createInputs(f cmdutil.Factory, buildFlags func(*cobra.Command), use string, short string, cueTemplateName string) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultNetWorkExample,
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
		Short: "network chaos.",
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

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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

		util.CheckErr(cmd.MarkFlagRequired("loss"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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

		util.CheckErr(cmd.MarkFlagRequired("latency"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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

		util.CheckErr(cmd.MarkFlagRequired("duplicate"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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

		util.CheckErr(cmd.MarkFlagRequired("corrupt"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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

// NewBandwidthCmd TODO
func NewBandwidthCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.BandwidthAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		cmd.Flags().StringVar(&o.Rate, "rate", "", `the rate at which the bandwidth is limited. For example : 10 bps/kbps/mbps/gbps.`)
		cmd.Flags().Uint32Var(&o.Limit, "limit", 1, `the number of bytes waiting in the queue.`)
		cmd.Flags().Uint32Var(&o.Buffer, "buffer", 1, `the maximum number of bytes that can be sent instantaneously.`)
		cmd.Flags().Uint64Var(&o.Peakrate, "peakrate", 0, `the maximum consumption rate of the bucket.`)
		cmd.Flags().Uint32Var(&o.Minburst, "minburst", 0, `the size of the peakrate bucket.`)

		util.CheckErr(cmd.MarkFlagRequired("rate"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
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
	cmd.Flags().StringVar(&o.TargetNamespaceSelector, "target-namespace-selector", "default", `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *NetworkChaosOptions) Validate() error {
	if o.TargetValue == "" && (o.TargetMode == "fixed" || o.TargetMode == "fixed-percent" || o.TargetMode == "random-max-percent") {
		return fmt.Errorf("you must use --value to specify an integer")
	}

	if ok, err := IsInteger(o.TargetValue); o.TargetValue != "" && !ok {
		return err
	}

	if ok, err := IsInteger(o.Loss); !ok {
		return err
	}

	if ok, err := IsInteger(o.Corrupt); !ok {
		return err
	}

	if ok, err := IsInteger(o.Duplicate); !ok {
		return err
	}

	if ok, err := IsInteger(o.Correlation); !ok {
		return err
	}

	if ok, err := IsRegularMatch(o.Latency); !ok {
		return err
	}

	if ok, err := IsRegularMatch(o.Jitter); !ok {
		return err
	}

	return o.BaseValidate()
}

func (o *NetworkChaosOptions) Complete() error {
	return o.BaseComplete()
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
