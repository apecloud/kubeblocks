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
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/util/prompt"
)

// DeleteHook is used to do some pre-delete or post-delete operations, for single deletion only.
type DeleteHook func(ctx context.Context, dynamic dynamic.Interface, namespace, name string) error

// DeleteOptions is the options for delete command.
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
	PreDeleteFn    DeleteHook
	PostDeleteFn   DeleteHook

	dynamic       dynamic.Interface
	deleteOptions *metav1.DeleteOptions
	listOptions   *metav1.ListOptions

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
	if err := o.validate(); err != nil {
		return err
	}

	if err := o.complete(); err != nil {
		return err
	}

	// delete results
	// return o.deleteResult(o.Result)
	return o.deleteResources()
}

func (o *DeleteOptions) validate() error {
	// names and label selector cannot be used together
	if len(o.Names) > 0 && len(o.LabelSelector) > 0 {
		return fmt.Errorf("name cannot be provided when a selector is specified")
	}
	// names and all namespaces cannot be used together
	if len(o.Names) > 0 && o.AllNamespaces {
		return fmt.Errorf("a resource cannot be retrieved by name across all namespaces")
	}
	if len(o.Names) == 0 && len(o.LabelSelector) == 0 {
		return fmt.Errorf("no name was specified. one of names, label selector must be provided")
	}
	// non-zero grace period cannot be used with immediate deletion
	switch {
	case o.GracePeriod == 0 && o.Force:
		fmt.Fprintf(o.ErrOut, "warning: Immediate deletion does not wait for confirmation that the running resource has been terminated.\n")
	case o.GracePeriod > 0 && o.Force:
		return fmt.Errorf("--force and --grace-period greater than 0 cannot be specified together")
	}
	// grace period cannot be used with now
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
	return nil
}

func (o *DeleteOptions) complete() error {
	var err error
	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if o.AllNamespaces {
		o.Namespace = metav1.NamespaceAll
	}

	o.dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	o.deleteOptions = &metav1.DeleteOptions{}
	if o.GracePeriod >= 0 {
		gracePeriod := (int64)(o.GracePeriod)
		o.deleteOptions.GracePeriodSeconds = &gracePeriod
	}
	o.listOptions = &metav1.ListOptions{}
	if len(o.LabelSelector) > 0 {
		o.listOptions.LabelSelector = o.LabelSelector
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
	return nil
}

func (o *DeleteOptions) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", false, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2). Matching objects must satisfy all of the specified label constraints.")
	cmd.Flags().BoolVar(&o.Force, "force", false, "If true, immediately remove resources from API and bypass graceful deletion. Note that immediate deletion of some resources may result in inconsistency or data loss and requires confirmation.")
	cmd.Flags().BoolVar(&o.Now, "now", false, "If true, resources are signaled for immediate shutdown (same as --grace-period=1).")
	cmd.Flags().IntVar(&o.GracePeriod, "grace-period", -1, "Period of time in seconds given to the resource to terminate gracefully. Ignored if negative. Set to 1 for immediate shutdown. Can only be set to 0 when --force is true (force deletion).")
	cmd.Flags().BoolVar(&o.AutoApprove, "auto-approve", false, "Skip interactive approval before deleting")
}

func (o *DeleteOptions) deleteResources() error {
	ctx := context.Background()
	var err error
	// handle label selector
	if len(o.LabelSelector) > 0 {
		if err = o.dynamic.Resource(o.GVR).Namespace(o.Namespace).DeleteCollection(ctx, *o.deleteOptions, *o.listOptions); err != nil {
			return err
		}
		return nil
	}
	// handle names
	for _, name := range o.Names {
		if o.PreDeleteFn != nil {
			if err = o.PreDeleteFn(ctx, o.dynamic, o.Namespace, name); err != nil {
				return err
			}
		}

		if err = o.dynamic.Resource(o.GVR).Namespace(o.Namespace).Delete(ctx, name, *o.deleteOptions); err != nil {
			return err
		}

		if o.PostDeleteFn != nil {
			if err = o.PostDeleteFn(ctx, o.dynamic, o.Namespace, name); err != nil {
				return err
			}
		}
	}
	return nil
}

// Confirm let user double-check what to delete
func Confirm(names []string, in io.Reader) error {
	if len(names) == 0 {
		return nil
	}

	entered, err := prompt.NewPrompt(fmt.Sprintf("You should type \"%s\"", strings.Join(names, " ")),
		"Please type the name again(separate with white space when more than one):", in).GetInput()
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
