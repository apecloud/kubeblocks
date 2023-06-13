package fault

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"strings"
)

type ListOptions struct {
	Resources []string
}

func NewListCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := cobra.Command{
		Use:     "list",
		Short:   "List all chaos resources.",
		Example: "# List all chaos resources \nkbcli fault list all",
		Run: func(cmd *cobra.Command, args []string) {

		},
	}
	return &cmd
}

func (o *ListOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		o.Resources = args
	}
	return nil
}

func (o *ListOptions) Validate() error {

	return nil
}

func (o *ListOptions) Run(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	//var err error
	if o.Resources == nil || len(o.Resources) == 0 {
		o.Resources, err = getAllChaosResources(f, Group+"/"+Version)
	}

	for _, resource := range o.Resources {
		if err := listResources(f, resource); err != nil {
			return err
		}
	}

	return nil
}

func listResources(f cmdutil.Factory, resource string) error {
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	gvr := GetGVR(Group, Version, resource)
	resourceList, err := dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.V(1).Info(err)
		return fmt.Errorf("failed to list %s: %s", gvr, err)
	}

	for _, obj := range resourceList.Items {
		fmt.Println(resource+":", obj.GetName())
	}

	return nil
}

func listAllResources() error {
	return nil
}

func getAllChaosResources(f cmdutil.Factory, groupVersion string) ([]string, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}
	chaosResources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		klog.V(1).Info(err)
		return nil, fmt.Errorf("failed to get server resources for %s: %s", groupVersion, err)
	}

	resourceNames := make([]string, 0)
	for _, resource := range chaosResources.APIResources {
		// skip subresources
		if len(strings.Split(resource.Name, "/")) > 1 {
			continue
		}
		// skip podhttpchaos and podnetworkchaos etc.
		if resource.Name != "podchaos" && strings.HasPrefix(resource.Name, "pod") {
			continue
		}
		resourceNames = append(resourceNames, resource.Name)
	}
	return resourceNames, nil
}
