/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package fault

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/pkg/cli/printer"
	"github.com/apecloud/kubeblocks/pkg/cli/util"
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
	Dynamic dynamic.Interface

	ResourceKinds    []string
	AllResourceKinds []string
	Kind             bool

	genericiooptions.IOStreams
}

func NewListCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &ListAndDeleteOptions{Factory: f, IOStreams: streams}
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

func NewDeleteCmd(f cmdutil.Factory, streams genericiooptions.IOStreams) *cobra.Command {
	o := &ListAndDeleteOptions{Factory: f, IOStreams: streams}
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
		return fmt.Errorf("failed to get all chaos resource kinds: %v", err)
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
			fmt.Fprintf(o.Out, "%s\n", resourceKind)
		}
		return nil
	}

	if len(args) > 0 {
		o.ResourceKinds = args
	} else {
		o.ResourceKinds = o.AllResourceKinds
	}

	var err error
	o.Dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %v", err)
	}

	return nil
}

func (o *ListAndDeleteOptions) RunList() error {
	if o.Kind {
		return nil
	}

	tbl := printer.NewTablePrinter(o.Out)
	tbl.Tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 2, WidthMax: 120},
	})
	tbl.SetHeader("NAME", "AGE")

	for _, resourceKind := range o.ResourceKinds {
		if err := o.listResources(resourceKind, tbl); err != nil {
			return err
		}
	}

	tbl.Print()
	return nil
}

func (o *ListAndDeleteOptions) RunDelete() error {
	for _, resourceKind := range o.ResourceKinds {
		if err := o.deleteResources(resourceKind); err != nil {
			return err
		}
	}
	return nil
}

func (o *ListAndDeleteOptions) listResources(resourceKind string, tbl *printer.TablePrinter) error {
	gvr := GetGVR(Group, Version, resourceKind)
	resourceList, err := o.Dynamic.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to list %s", gvr)
	}

	if len(resourceList.Items) == 0 {
		return nil
	}

	// sort by creation time from old to new
	sort.Slice(resourceList.Items, func(i, j int) bool {
		t1, _ := time.Parse(time.RFC3339, resourceList.Items[i].GetCreationTimestamp().String())
		t2, _ := time.Parse(time.RFC3339, resourceList.Items[j].GetCreationTimestamp().String())
		return t1.Before(t2)
	})

	for _, obj := range resourceList.Items {
		creationTime := obj.GetCreationTimestamp().Time
		age := time.Since(creationTime).Round(time.Second).String()
		tbl.AddRow(obj.GetName(), age)
	}
	return nil
}

func (o *ListAndDeleteOptions) deleteResources(resourceKind string) error {
	gvr := GetGVR(Group, Version, resourceKind)
	resourceList, err := o.Dynamic.Resource(gvr).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to list %s", gvr)
	}

	if len(resourceList.Items) == 0 {
		return nil
	}

	for _, obj := range resourceList.Items {
		err = o.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Delete(context.TODO(), obj.GetName(), metav1.DeleteOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to delete %s", gvr)
		}
		fmt.Fprintf(o.Out, "delete resource %s/%s\n", obj.GetNamespace(), obj.GetName())
	}
	return nil
}

func getAllChaosResourceKinds(f cmdutil.Factory, groupVersion string) ([]string, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create discovery client")
	}
	chaosResources, err := discoveryClient.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get server resources for %s", groupVersion)
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
