package fault

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"

	"k8s.io/kubectl/pkg/util/templates"

	"k8s.io/apimachinery/pkg/runtime"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/leaanthony/debme"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

var faultPodExample = templates.Examples(`
	"kbcli fault pod kill --mode=fixed --value=2 --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2",

	"kbcli fault pod kill-container --container-names=mysql --label=statefulset.kubernetes.io/pod-name=mycluster-mysql-2",
`)

const (
	CueTemplateName = "podChaos_template.cue"
	PodKill         = "pod-kill"
	PodFailure      = "pod-failure"
	ContainerKill   = "container-kill"
)

type FaultOptions struct {
	Factory cmdutil.Factory `json:"-"`

	Namespace string `json:"namespace"`

	Dynamic dynamic.Interface `json:"-"`

	Client kubernetes.Interface `json:"-"`

	ClientSet kubernetes.Interface `json:"-"`

	Action string `json:"action"`

	Mode string `json:"mode"`
	// Value The number and percentage of fault injection pods
	Value string `json:"value"`

	NamespaceSelector string `json:"namespaceSelector"`

	Label map[string]string `json:"label,omitempty"`
	// GracePeriod waiting time, after which fault injection is performed
	GracePeriod int `json:"gracePeriod"`
	// Duration the duration of the Pod Failure experiment
	Duration string `json:"duration"`

	ContainerNames []string `json:"containerNames,omitempty"`

	DryRun string `json:"-"`

	CueTemplateName string `json:"-"`

	ToPrinter func(*meta.RESTMapping, bool) (printers.ResourcePrinterFunc, error) `json:"-"`

	Format printer.Format `json:"-"`

	genericclioptions.IOStreams
}

func NewFaultPodCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pod",
		Short:   "inject fault to pod.",
		Example: faultPodExample,
	}
	cmd.AddCommand(
		NewPodKillCmd(f, streams),
		NewPodFailureCmd(f, streams),
		NewContainerKillCmd(f, streams),
	)
	return cmd
}

func NewPodKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &FaultOptions{
		Factory:         f,
		IOStreams:       streams,
		Action:          PodKill,
		CueTemplateName: CueTemplateName,
	}
	cmd := &cobra.Command{
		Use:     "kill",
		Short:   "Kill pod",
		Example: faultPodExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.validate(cmd, args))
			util.CheckErr(o.run())
		},
	}
	AddCommonFlag(cmd, o)
	cmd.Flags().IntVar(&o.GracePeriod, "grace-period", 0, "Grace period represents the duration in seconds before the pod should be deleted")

	util.CheckErr(cmd.MarkFlagRequired("label"))
	return cmd
}

func NewPodFailureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &FaultOptions{
		Factory:         f,
		IOStreams:       streams,
		Action:          PodFailure,
		CueTemplateName: CueTemplateName,
	}
	cmd := &cobra.Command{
		Use:     "failure",
		Short:   "Failure pod",
		Example: faultPodExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.validate(cmd, args))
			util.CheckErr(o.run())
		},
	}
	AddCommonFlag(cmd, o)
	cmd.Flags().StringVar(&o.Duration, "duration", "10s", "Supported formats of the duration are: ms / s / m / h.")

	util.CheckErr(cmd.MarkFlagRequired("label"))
	return cmd
}

func NewContainerKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &FaultOptions{
		Factory:         f,
		IOStreams:       streams,
		Action:          ContainerKill,
		CueTemplateName: CueTemplateName,
	}
	cmd := &cobra.Command{
		Use:     "kill-container",
		Short:   "Kill container",
		Example: faultPodExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete())
			util.CheckErr(o.validate(cmd, args))
			util.CheckErr(o.run())
		},
	}
	AddCommonFlag(cmd, o)
	cmd.Flags().StringArrayVar(&o.ContainerNames, "container-names", nil, "input")

	util.CheckErr(cmd.MarkFlagRequired("container-names"))
	util.CheckErr(cmd.MarkFlagRequired("label"))
	return cmd
}

func AddCommonFlag(cmd *cobra.Command, o *FaultOptions) {

	cmd.Flags().StringVar(&o.Mode, "mode", "one", `You can select "one", "all", "fixed", "fixed-percent", "random-max-percent", Specify the experimental mode, that is, which Pods to experiment with.`)
	cmd.Flags().StringVar(&o.Value, "value", "2", `If you choose mode=fixed or fixed-percent, you can enter a value to specify the number of pods you want to inject.`)

	cmd.Flags().StringToStringVar(&o.Label, "label", nil, `label for pod, such as '"app.kubernetes.io/component=mysql, statefulset.kubernetes.io/pod-name=mycluster-mysql-0"'`)
	cmd.Flags().StringVar(&o.NamespaceSelector, "namespace-selector", "", `Specifies the namespace into which you want to inject faults.`)

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = "unchanged"

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *FaultOptions) complete() error {

	var err error
	if o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	if o.Dynamic, err = o.Factory.DynamicClient(); err != nil {
		return err
	}

	if o.Client, err = o.Factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.ClientSet, err = o.Factory.KubernetesClientSet(); err != nil {
		return err
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, withNamespace bool) (printers.ResourcePrinterFunc, error) {
		var p printers.ResourcePrinter
		switch o.Format {
		case printer.JSON:
			p = &printers.JSONPrinter{}
		case printer.YAML:
			p = &printers.YAMLPrinter{}
		default:
			return nil, genericclioptions.NoCompatiblePrinterError{AllowedFormats: []string{"JSON", "YAML"}}
		}

		p, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(p, nil)
		if err != nil {
			return nil, err
		}
		return p.PrintObj, nil
	}

	return nil
}

func (o *FaultOptions) validate(cmd *cobra.Command, args []string) error {

	return nil
}

func (o *FaultOptions) PreCreate(obj *unstructured.Unstructured) error {
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

func (o *FaultOptions) run() error {

	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(&o); err != nil {
		return err
	}

	if cueValue, err = newCueValue(o.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = convertContentToUnstructured(cueValue); err != nil {
		return err
	}
	if o.DryRun == "client" {
		printer, err := o.ToPrinter(nil, false)
		if err != nil {
			return err
		}
		return printer.PrintObj(unstructuredObj, o.Out)
	}
	gvr := schema.GroupVersionResource{Group: "chaos-mesh.org", Version: "v1alpha1", Resource: "podchaos"}
	unstructuredObj, err = o.Dynamic.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	return nil
}

// NewCueValue convert cue template  to cue Value which holds any value like Boolean,Struct,String and more cue type.
func newCueValue(cueTemplateName string) (cue.Value, error) {
	tmplFs, _ := debme.FS(cueTemplate, "template")
	if tmlBytes, err := tmplFs.ReadFile(cueTemplateName); err != nil {
		return cue.Value{}, err
	} else {
		return cuecontext.New().CompileString(string(tmlBytes)), nil
	}
}

// fillOptions fill options object in cue template file
func fillOptions(cueValue cue.Value, optionsByte []byte) (cue.Value, error) {
	var (
		expr ast.Expr
		err  error
	)
	if expr, err = cuejson.Extract("", optionsByte); err != nil {
		return cue.Value{}, err
	}
	optionsValue := cueValue.Context().BuildExpr(expr)
	cueValue = cueValue.FillPath(cue.ParsePath("options"), optionsValue)
	return cueValue, nil
}

// convertContentToUnstructured get content object in cue template file and convert it to Unstructured
func convertContentToUnstructured(cueValue cue.Value) (*unstructured.Unstructured, error) {
	var (
		contentByte     []byte
		err             error
		unstructuredObj = &unstructured.Unstructured{}
	)
	if contentByte, err = cueValue.LookupPath(cue.ParsePath("content")).MarshalJSON(); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(contentByte, &unstructuredObj); err != nil {
		return nil, err
	}
	return unstructuredObj, nil
}
