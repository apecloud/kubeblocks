package fault

import (
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type NetworkAttackOptions struct {
	Direction string `json:"direction"`

	TargetLabel map[string]string `json:"targetLabel"`

	TargetNamespaceSelector string `json:"targetNamespaceSelector"`

	TargetMode string `json:"targetMode"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *NetworkAttackOptions) createInputs(f cmdutil.Factory, use string, short string, BuildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultPodExample,
		CueTemplateName: CueTemplateNetworkChaosName,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourceNetworkChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      BuildFlags,
	}
}

func NewNetworkAttackCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
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
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PartitionAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Partition,
		PartitionShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewLossCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.LossAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Loss,
		LossShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewDelayCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.DelayAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Delay,
		DelayShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewDuplicateCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.DuplicateAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Duplicate,
		DuplicateShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewCorruptCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.CorruptAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Corrupt,
		CorruptShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewBandwidthCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &NetworkAttackOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.BandwidthAction)},
	}
	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		util.CheckErr(cmd.MarkFlagRequired("label"))
	}
	inputs := o.createInputs(
		f,
		Bandwidth,
		BandwidthShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *NetworkAttackOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "one", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "1", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)

	cmd.Flags().IntVar(&o.GracePeriod, "grace-period", 0, "Grace period represents the duration in seconds before the pod should be killed")

	cmd.Flags().StringToStringVar(&o.Label, "label", nil, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringVar(&o.NamespaceSelector, "namespace-selector", "", `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().String("dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *NetworkAttackOptions) Validate() error {
	err := o.BaseValidate()
	if err != nil {
		return err
	}
	return nil
}

func (o *NetworkAttackOptions) Complete() error {
	err := o.BaseComplete()
	if err != nil {
		return err
	}
	return nil
}

func (o *NetworkAttackOptions) PreCreate(obj *unstructured.Unstructured) error {
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
