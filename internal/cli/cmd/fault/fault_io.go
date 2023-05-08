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

var faultIOExample = templates.Examples(`
	# Injects a delay fault into the /data directory, causing a 10-second delay for all filesystem operations (including reading, writing, listing directory contents, etc.) under this directory.
	kbcli fault IO latency --delay=10s --volume-path=/data
	
	# Inject a file error fault into the /data directory, so that all file system operations under this directory have a 100% probability of error and return error code 22 (Invalid argument).
	kbcli fault IO fault --volume-path=/data --errno=22
	
	# Inject the attrOverride fault into the /data directory, so that all file system operations in this directory will have a 100% probability of changing the permission of the target file to 72 (that is, 110 in octal), which will make the file only accessible by the owner and the location Executed by the group and has no right to perform other operations.
	kbcli fault IO attribute --volume-path=/data --perm=72
	
	# Inject read and write error faults into the /data directory, so that read and write operations under this directory will have a 100% probability of error. Among them, random positions of 1 with a maximum length of 10 in bytes will be replaced with 0.
	kbcli fault IO mistake --volume-path=/data --filling=zero --maxOccurrences=10 --maxLength=1
`)

type IOChaosOptions struct {
	// Parameters required by the `latency` command.
	Delay string `json:"delay"`

	// Parameters required by the `fault` command.
	Errno int `json:"errno"`

	// Parameters required by the `attribute` command.
	Ino    uint64 `json:"ino,omitempty"`
	Size   uint64 `json:"size,omitempty"`
	Blocks uint64 `json:"blocks,omitempty"`
	Perm   uint16 `json:"perm,omitempty"`
	Nlink  uint32 `json:"nlink,omitempty"`
	UID    uint32 `json:"uid,omitempty"`
	GID    uint32 `json:"gid,omitempty"`

	// Parameters required by the `mistake` command.
	Filling        string `json:"filling,omitempty"`
	MaxOccurrences int    `json:"maxOccurrences"`
	MaxLength      int    `json:"maxLength"`

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
		NewIOAttributeOverrideCmd(f, streams),
		NewIOMistakeCmd(f, streams),
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
		cmd.Flags().StringVar(&o.Delay, "delay", "", `Specific delay time.`)

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
		cmd.Flags().IntVar(&o.Errno, "errno", 0, `The returned error number.`)

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

func NewIOAttributeOverrideCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &IOChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.IoAttrOverride)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		cmd.Flags().Uint64Var(&o.Ino, "ino", 0, `ino number.`)
		cmd.Flags().Uint64Var(&o.Size, "size", 0, `File size.`)
		cmd.Flags().Uint64Var(&o.Blocks, "blocks", 0, `The number of blocks the file occupies.`)
		cmd.Flags().Uint16Var(&o.Perm, "perm", 0, `Decimal representation of file permissions.`)
		cmd.Flags().Uint32Var(&o.Nlink, "nlink", 0, `The number of hard links.`)
		cmd.Flags().Uint32Var(&o.UID, "uid", 0, `Owner's user ID.`)
		cmd.Flags().Uint32Var(&o.GID, "gid", 0, `The owner's group ID.`)

		util.CheckErr(cmd.MarkFlagRequired("volume-path"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Attribute,
		AttributeShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func NewIOMistakeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &IOChaosOptions{
		BaseOptions:      create.BaseOptions{IOStreams: streams},
		FaultBaseOptions: FaultBaseOptions{Action: string(v1alpha1.IoMistake)},
	}

	var BuildFlags = func(cmd *cobra.Command) {
		o.AddCommonFlag(cmd)

		cmd.Flags().StringVar(&o.Filling, "filling", "", `The filling content of the error data can only be zero (filling with 0) or random (filling with random bytes).`)
		cmd.Flags().IntVar(&o.MaxOccurrences, "maxOccurrences", 1, `The maximum number of times an error can occur per operation.`)
		cmd.Flags().IntVar(&o.MaxLength, "maxLength", 1, `The maximum length (in bytes) of each error.`)

		util.CheckErr(cmd.MarkFlagRequired("filling"))
		util.CheckErr(cmd.MarkFlagRequired("maxOccurrences"))
		util.CheckErr(cmd.MarkFlagRequired("maxLength"))
		util.CheckErr(cmd.MarkFlagRequired("volume-path"))

		// register flag completion func
		registerFlagCompletionFunc(cmd, f)
	}
	inputs := o.createInputs(
		f,
		Mistake,
		MistakeShort,
		BuildFlags,
	)

	return create.BuildCommand(*inputs)
}

func (o *IOChaosOptions) AddCommonFlag(cmd *cobra.Command) {

	cmd.Flags().StringVar(&o.Mode, "mode", "all", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "", `If you choose mode=fixed or fixed-percent or random-max-percent, you can enter a value to specify the number or percentage of pods you want to inject.`)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringToStringVar(&o.Label, "label", map[string]string{}, `Label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringArrayVar(&o.NamespaceSelector, "namespace-selector", []string{"default"}, `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.VolumePath, "volume-path", "", `The mount point of the volume in the target container must be the root directory of the mount.`)
	cmd.Flags().StringVar(&o.Path, "path", "", `The effective scope of the injection error can be a wildcard or a single file.`)
	cmd.Flags().IntVar(&o.Percent, "percent", 100, `Probability of failure per operation, in %.`)
	cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, "The name of the container, such as mysql, prometheus.If it's empty, the first container will be injected.")
	cmd.Flags().StringArrayVar(&o.Methods, "methods", nil, "The file system calls that need to inject faults. For example: WRITE READ")

	cmd.Flags().StringVar(&o.DryRunStrategy, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *IOChaosOptions) Validate() error {
	return o.BaseValidate()
}

func (o *IOChaosOptions) Complete() error {
	return o.BaseComplete()
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
