package fault

import (
	"fmt"
	"strings"

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
	# --label is required to specify the pods that need to be killed. 
	kbcli fault pod kill --label statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2
	
	kbcli fault pod kill --mode=fixed --value=2 --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2

	kbcli fault pod kill-container --container-names=mysql --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2
`)

type PodOptions struct {
	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions

	create.BaseOptions
}

func NewFaultPodCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod",
		Short: "inject fault to pod.",
	}
	cmd.AddCommand(
		NewPodKillCmd(f, streams),
		NewPodFailureCmd(f, streams),
		NewContainerKillCmd(f, streams),
	)
	return cmd
}

func NewPodKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PodKillAction)},
	}
	inputs := create.Inputs{
		Use:             "kill",
		Short:           "kill a pod.",
		Example:         faultPodExample,
		CueTemplateName: CueTemplatePodChaosName,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourcePodChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags: func(cmd *cobra.Command) {
			o.AddCommonFlag(cmd)

			util.CheckErr(cmd.MarkFlagRequired("label"))

			// register flag completion func
			registerFlagCompletionFunc(cmd, f)
		},
	}

	return create.BuildCommand(inputs)
}

func NewPodFailureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.PodFailureAction)},
	}
	inputs := create.Inputs{
		Use:             "failure",
		Short:           "failure a pod.",
		Example:         faultPodExample,
		CueTemplateName: CueTemplatePodChaosName,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourcePodChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags: func(cmd *cobra.Command) {
			o.AddCommonFlag(cmd)
			cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")

			util.CheckErr(cmd.MarkFlagRequired("label"))
			util.CheckErr(cmd.MarkFlagRequired("duration"))

			// register flag completion func
			registerFlagCompletionFunc(cmd, f)
		},
	}

	return create.BuildCommand(inputs)
}

func NewContainerKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PodOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.ContainerKillAction)},
	}
	inputs := create.Inputs{
		Use:             "kill-container",
		Short:           "kill a container.",
		Example:         faultPodExample,
		CueTemplateName: CueTemplatePodChaosName,
		Group:           Group,
		Version:         Version,
		ResourceName:    ResourcePodChaos,
		BaseOptionsObj:  &o.BaseOptions,
		Options:         o,
		Factory:         f,
		Validate:        o.Validate,
		Complete:        o.Complete,
		PreCreate:       o.PreCreate,
		BuildFlags: func(cmd *cobra.Command) {

			o.AddCommonFlag(cmd)
			cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, "the name of the container you want to kill, such as mysql, prometheus.")

			util.CheckErr(cmd.MarkFlagRequired("container-names"))
			util.CheckErr(cmd.MarkFlagRequired("label"))

			// register flag completion func
			registerFlagCompletionFunc(cmd, f)
		},
	}

	return create.BuildCommand(inputs)
}

func registerFlagCompletionFunc(cmd *cobra.Command, f cmdutil.Factory) {
	var formatsWithDesc = map[string]string{
		"JSON": "Output result in JSON format",
		"YAML": "Output result in YAML format",
	}
	util.CheckErr(cmd.RegisterFlagCompletionFunc("output",
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for format, desc := range formatsWithDesc {
				if strings.HasPrefix(format, toComplete) {
					names = append(names, fmt.Sprintf("%s\t%s", format, desc))
				}
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}))
}

func (o *PodOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "one", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "1", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)

	cmd.Flags().IntVar(&o.GracePeriod, "grace-period", 0, "Grace period represents the duration in seconds before the pod should be killed")

	cmd.Flags().StringToStringVar(&o.Label, "label", nil, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringVar(&o.NamespaceSelector, "namespace-selector", "", `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().String("dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *PodOptions) Validate() error {
	err := o.BaseValidate()
	if err != nil {
		return err
	}
	return nil
}

func (o *PodOptions) Complete() error {
	err := o.BaseComplete()
	if err != nil {
		return err
	}
	return nil
}

func (o *PodOptions) PreCreate(obj *unstructured.Unstructured) error {
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
