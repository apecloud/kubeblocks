package fault

import (
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var faultPodExample = templates.Examples(`
	# kill all pods in default namespace
	kbcli fault pod kill
	
	# kill any pod in default namespace
	kbcli fault pod kill --mode=one

	# kill two pods in default namespace
	kbcli fault pod kill --mode=fixed --value=2

	# --label is required to specify the pods that need to be killed. 
	kbcli fault pod kill --label statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2
	
	kbcli kbcli fault pod failure --duration=1m

	# kill container in pod
	kbcli fault pod kill-container --container-names=mysql --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2
`)

type PodChaosOptions struct {
	// GracePeriod waiting time, after which fault injection is performed
	GracePeriod    int      `json:"gracePeriod"`
	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions

	create.BaseOptions
}

func (o *PodChaosOptions) createInputs(f cmdutil.Factory, use string, short string, buildFlags func(*cobra.Command)) *create.Inputs {
	return &create.Inputs{
		Use:             use,
		Short:           short,
		Example:         faultPodExample,
		CueTemplateName: CueTemplatePodChaos,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourcePodChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags:      buildFlags,
	}
}

func NewPodChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod",
		Short: "pod chaos.",
	}
	cmd.AddCommand(
		NewPodKillCmd(f, streams),
		NewPodFailureCmd(f, streams),
		NewContainerKillCmd(f, streams),
	)
	return cmd
}

func NewPodKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PodKillAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)
		cmd.Flags().IntVar(&o.GracePeriod, "grace-period", 0, "Grace period represents the duration in seconds before the pod should be killed")

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Kill,
		KillShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewPodFailureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PodFailureAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		util.CheckErr(cmd.MarkFlagRequired("duration"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Failure,
		FailureShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewContainerKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.ContainerKillAction)},
	}

	var BuildFlags = func(cmd *cobra.Command) {

		o.AddCommonFlag(cmd)
		cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, "the name of the container you want to kill, such as mysql, prometheus.")

		util.CheckErr(cmd.MarkFlagRequired("container-names"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		KillContainer,
		KillContainerShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *PodChaosOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *PodChaosOptions) Validate() error {
	return o.BaseValidate()
}

func (o *PodChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *PodChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.PodChaos{}
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
