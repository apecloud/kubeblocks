package fault

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var listExample = templates.Examples(`
	# List all chaos resources
	kbcli fault list
	
	# List all chaos kind
	kbcli fault list --kind

	# List specific chaos resources. Use 'kbcli fault list --kind' to get chaos kind. 
	kbcli fault list podchaos
`)

var deleteExample = templates.Examples(`
	# Delete all chaos resources
	kbcli fault delete
	
	# Delete specific chaos resources
	kbcli fault delete podchaos
`)

type ListAndDeleteOptions struct {
	Factory cmdutil.Factory

	ResourceKinds    []string
	AllResourceKinds []string
	Kind             bool
}

func NewListCmd(f cmdutil.Factory) *cobra.Command {
	o := &ListAndDeleteOptions{Factory: f}
	cmd := cobra.Command{
		Use:     "list",
		Short:   "List chaos resources.",
		Example: listExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Validate(args))
			util.CheckErr(o.Complete(args))
			util.CheckErr(o.RunList())
		},
	}
	cmd.Flags().BoolVar(&o.Kind, "kind", false, "Print chaos resource kind.")
	return &cmd
}

func NewDeleteCmd(f cmdutil.Factory) *cobra.Command {
	o := &ListAndDeleteOptions{Factory: f}
	return &cobra.Command{
		Use:     "delete",
		Short:   "Delete chaos resources.",
		Example: deleteExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.Validate(args))
			util.CheckErr(o.Complete(args))
			util.CheckErr(o.RunDelete())
		},
	}
}

func (o *ListAndDeleteOptions) Validate(args []string) error {
	var err error
	o.AllResourceKinds, err = getAllChaosResourceKinds(o.Factory, GroupVersion)
	if err != nil {
		return fmt.Errorf("failed to get all chaos resources: %s", err)
	}

	kindMap := make(map[string]bool)
	for _, kind := range o.AllResourceKinds {
		kindMap[kind] = true
	}
	for _, kind := range args {
		if _, ok := kindMap[kind]; !ok {
			return fmt.Errorf("invalid chaos resource kind: %s\nUse 'kbcli fault list --kind' to list all chaos resource kinds", kind)
		}
	}

	return nil
}

func (o *ListAndDeleteOptions) Complete(args []string) error {
	if o.Kind {
		for _, resourceKind := range o.AllResourceKinds {
			fmt.Println(resourceKind)
		}
		return nil
	}

	if len(args) > 0 {
		o.ResourceKinds = args
	} else {
		o.ResourceKinds = o.AllResourceKinds
	}
	return nil
}

func (o *ListAndDeleteOptions) RunList() error {
	fmt.Printf("%-20s %s\n", "NAME", "AGE")
	for _, resourceKind := range o.ResourceKinds {
		if err := listResources(o.Factory, resourceKind); err != nil {
			return err
		}
	}
	return nil
}

func (o *ListAndDeleteOptions) RunDelete() error {
	for _, resourceKind := range o.ResourceKinds {
		if err := deleteResources(o.Factory, resourceKind); err != nil {
			return err
		}
	}
	return nil
}

func listResources(f cmdutil.Factory, resourceKind string) error {
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	gvr := GetGVR(Group, Version, resourceKind)
	resourceList, err := dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.V(1).Info(err)
		return fmt.Errorf("failed to list %s: %s", gvr, err)
	}

	// sort by creation time form old to new
	sort.Slice(resourceList.Items, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, resourceList.Items[i].GetCreationTimestamp().String())
		t2, _ := time.Parse(time.RFC3339, resourceList.Items[j].GetCreationTimestamp().String())
		return t1.Before(t2)
	})

	for _, obj := range resourceList.Items {
		creationTime := obj.GetCreationTimestamp().Time
		age := time.Since(creationTime).Round(time.Second).String()
		fmt.Printf("%-20s %s\n", obj.GetName(), age)
	}

	return nil
}

func deleteResources(f cmdutil.Factory, resourceKind string) error {
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	gvr := GetGVR(Group, Version, resourceKind)
	resourceList, err := dynamicClient.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		klog.V(1).Info(err)
		return fmt.Errorf("failed to list %s: %s", gvr, err)
	}

	for _, obj := range resourceList.Items {
		err = dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			klog.V(1).Info(err)
			return fmt.Errorf("failed to delete %s: %s", gvr, err)
		}
		fmt.Println("delete resource", obj.GetName())
	}
	return nil
}

func getAllChaosResourceKinds(f cmdutil.Factory, groupVersion string) ([]string, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery client: %v", err)
	}
	chaosResources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		klog.V(1).Info(err)
		return nil, fmt.Errorf("failed to get server resources for %s: %s", groupVersion, err)
	}

	resourceKinds := make([]string, 0)
	for _, resourceKind := range chaosResources.APIResources {
		// skip subresources
		if len(strings.Split(resourceKind.Name, "/")) > 1 {
			continue
		}
		// skip podhttpchaos and podnetworkchaos etc.
		if resourceKind.Name != "podchaos" && strings.HasPrefix(resourceKind.Name, "pod") {
			continue
		}
		resourceKinds = append(resourceKinds, resourceKind.Name)
	}
	return resourceKinds, nil
}
