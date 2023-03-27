package builder

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type builderCmdOpts struct {
	genericclioptions.IOStreams

	Factory cmdutil.Factory
	// dynamic dynamic.Interface

	clusterDefName string
	groupSelectors []string

	complete func(self *builderCmdOpts, cmd *cobra.Command, args []string) error
}

// NewBuilderCmd for builder functions
func NewBuilderCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "builder",
		Short: "builder command.",
	}
	cmd.AddCommand(
		newCreateClusterDefCmd(f, streams),
	)
	return cmd
}

func newCreateClusterDefCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &builderCmdOpts{
		// Options:   patch.NewOptions(f, streams, types.AddonGVR()),
		Factory:   f,
		IOStreams: streams,
		complete:  createClusterDefHandler,
	}
	cmd := &cobra.Command{
		Use:   "create-cluster-def {COMPONENT_DISCOVERY_YAML file}",
		Short: "Create ClusterDefinition API YAML from a component discovery Spec. YAML file.",
		Example: templates.Examples(`
    	# Create ClusterDefinition API spec YAML from my-component-discovery.yaml discovery input spec. file.
    	kbcli builder create-cluster-def my-component-discovery.yaml
`),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(o, cmd, args))
		},
	}
	cmd.Flags().StringVar(&o.clusterDefName, "cluster-def-name", "",
		"Set ClusterDefinition API object name")
	cmd.Flags().StringArrayVar(&o.groupSelectors, "group", []string{},
		"Component group selector")
	return cmd
}

func createClusterDefHandler(o *builderCmdOpts, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("invalid arg")
	}

	_, err := os.Stat(args[0])
	if err != nil {
		return err
	}

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	compDiscovery := &appsv1alpha1.ComponentsDiscovery{}
	if err = yaml.Unmarshal(b, compDiscovery); err != nil {
		return err
	}

	// TODO: validate CR with CRD OpenAPI schema validation
	// gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: inputs.ResourceName}
	// if unstructuredObj, err = o.Dynamic.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
	// 	return err
	// }

	switch compDiscovery.Spec.Type {
	case appsv1alpha1.HelmDiscoveryType:
		helmSpec := compDiscovery.Spec.Helm
		if helmSpec == nil {
			return errors.New("missing required `spec.helm` value")
		}
		if helmSpec.ChartLocationURL == "" {
			return errors.New("missing required `spec.helm.chartLocationURL` value")
		}

		

	case appsv1alpha1.ExistingClusterDiscoveryType:
	case appsv1alpha1.ManifestsDiscoveryType:
	}

	return nil
}
