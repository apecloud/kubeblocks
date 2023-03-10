/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package delete

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

type DeleteOptions struct {
	Factory       cmdutil.Factory
	Namespace     string
	LabelSelector string
	AllNamespaces bool
	Force         bool
	GracePeriod   int
	Now           bool
	AutoApprove   bool

	// Names are the resource names
	Names []string
	// ConfirmedNames used to double-check the resource names to delete, sometimes Names are used to build
	// label selector and be set to nil, ConfirmedNames should be used to record the names to be confirmed.
	ConfirmedNames []string
	GVR            schema.GroupVersionResource
	Result         *resource.Result

	genericclioptions.IOStreams
}

func NewDeleteOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, gvr schema.GroupVersionResource) *DeleteOptions {
	return &DeleteOptions{
		Factory:   f,
		IOStreams: streams,
		GVR:       gvr,
	}
}

func (o *DeleteOptions) Run() error {
	if err := o.complete(); err != nil {
		return err
	}

	// delete results
	return o.deleteResult(o.Result)
}

func (o *DeleteOptions) complete() error {
	switch {
	case o.GracePeriod == 0 && o.Force:
		fmt.Fprintf(o.ErrOut, "warning: Immediate deletion does not wait for confirmation that the running resource has been terminated.\n")
	case o.GracePeriod > 0 && o.Force:
		return fmt.Errorf("--force and --grace-period greater than 0 cannot be specified together")
	}

	if o.Now {
		if o.GracePeriod != -1 {
			return fmt.Errorf("--now and --grace-period cannot be specified together")
		}
		o.GracePeriod = 1
	}
	if o.GracePeriod == 0 && !o.Force {
		o.GracePeriod = 1
	}
	if o.Force && o.GracePeriod < 0 {
		o.GracePeriod = 0
	}

	namespace, _, err := o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// confirm names to delete, use ConfirmedNames first, if it is empty, use Names
	if !o.AutoApprove {
		names := o.ConfirmedNames
		if len(names) == 0 {
			names = o.Names
		}
		if err = Confirm(names, o.In); err != nil {
			return err
		}
	}

	// get the resources to delete
	r := o.Factory.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(namespace).DefaultNamespace().
		LabelSelectorParam(o.LabelSelector).
		AllNamespaces(o.AllNamespaces).
		ResourceTypeOrNameArgs(false, append([]string{util.GVRToString(o.GVR)}, o.Names...)...).
		RequireObject(false).
		Flatten().
		Do()
	err = r.Err()
	if err != nil {
		return err
	}
	o.Result = r
	return err
}

func (o *DeleteOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	cmd.Flags().BoolVar(&o.Force, "force", false, "If true, immediately remove resources from API and bypass graceful deletion. Note that immediate deletion of some resources may result in inconsistency or data loss and requires confirmation.")
	cmd.Flags().BoolVar(&o.Now, "now", false, "If true, resources are signaled for immediate shutdown (same as --grace-period=1).")
	cmd.Flags().IntVar(&o.GracePeriod, "grace-period", -1, "Period of time in seconds given to the resource to terminate gracefully. Ignored if negative. Set to 1 for immediate shutdown. Can only be set to 0 when --force is true (force deletion).")
	cmd.Flags().BoolVar(&o.AutoApprove, "auto-approve", false, "Skip interactive approval before deleting")
}

func (o *DeleteOptions) deleteResult(r *resource.Result) error {
	found := 0
	var deleteInfos []*resource.Info
	err := r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		deleteInfos = append(deleteInfos, info)
		found++

		options := &metav1.DeleteOptions{}
		if o.GracePeriod >= 0 {
			options = metav1.NewDeleteOptions(int64(o.GracePeriod))
		}
		if _, err = o.deleteResource(info, options); err != nil {
			return err
		}
		fmt.Fprintf(o.Out, "%s %s deleted\n", info.Mapping.GroupVersionKind.Kind, info.Name)
		return nil
	})
	if err != nil {
		return err
	}
	if found == 0 {
		fmt.Fprintf(o.Out, "No %s found\n", o.GVR.Resource)
	}

	return nil
}

func (o *DeleteOptions) deleteResource(info *resource.Info, deleteOptions *metav1.DeleteOptions) (runtime.Object, error) {
	response, err := resource.
		NewHelper(info.Client, info.Mapping).
		DryRun(false).
		DeleteWithOptions(info.Namespace, info.Name, deleteOptions)
	if err != nil {
		return nil, cmdutil.AddSourceToErr("deleting", info.Source, err)
	}
	return response, nil
}

// Confirm let user double-check what to delete
func Confirm(names []string, in io.Reader) error {
	if len(names) == 0 {
		return nil
	}

	entered, err := prompt.NewPrompt(fmt.Sprintf("You should type \"%s\"", strings.Join(names, " ")),
		"Please type the cluster name again(separate with white space when more than one):", in).GetInput()
	if err != nil {
		return err
	}
	enteredNames := strings.Split(entered, " ")
	sort.Strings(names)
	sort.Strings(enteredNames)
	if !slices.Equal(names, enteredNames) {
		return fmt.Errorf("typed \"%s\" does not match \"%s\"", entered, strings.Join(names, " "))
	}
	return nil
}
