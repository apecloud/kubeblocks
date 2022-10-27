/*
Copyright 2022 The KubeBlocks Authors

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

package describe

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/describe"
)

// Options used to construct a describe command
type Options struct {
	Factory   cmdutil.Factory
	Short     string
	Namespace string
	Selector  string

	Describer func(*meta.RESTMapping) (describe.ResourceDescriber, error)
	Builder   func() *resource.Builder

	GroupKind schema.GroupKind
	Name      string

	EnforceNamespace bool
	AllNamespaces    bool

	DescriberSettings *describe.DescriberSettings
	genericclioptions.IOStreams
}

func NewOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, short string, gk schema.GroupKind) *Options {
	return &Options{
		Factory:   f,
		IOStreams: streams,
		Short:     short,
		GroupKind: gk,
		DescriberSettings: &describe.DescriberSettings{
			ShowEvents: true,
			ChunkSize:  cmdutil.DefaultChunkSize,
		},
	}
}

// Build return a describe command
func (o *Options) Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: o.Short,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.complete(args))
			cmdutil.CheckErr(o.run())
		},
	}

	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().BoolVar(&o.AllNamespaces, "all-namespaces", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.DescriberSettings.ShowEvents, "show-events", o.DescriberSettings.ShowEvents, "If true, display events related to the described object.")

	return cmd
}

func (o *Options) complete(args []string) error {
	var err error
	if len(args) == 0 || o.GroupKind.Empty() {
		return errors.New("You must specify the name of resource to describe.")
	}
	o.Name = args[0]
	if o.Namespace, o.EnforceNamespace, err = o.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}
	if o.AllNamespaces {
		o.EnforceNamespace = false
	}

	o.Describer = func(mapping *meta.RESTMapping) (describe.ResourceDescriber, error) {
		return DescriberFn(o.Factory, mapping)
	}
	o.Builder = o.Factory.NewBuilder
	return nil
}

func (o *Options) run() error {
	r := o.Builder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		LabelSelectorParam(o.Selector).
		ResourceTypeOrNameArgs(true, o.GroupKind.String(), o.Name).
		Flatten().
		Do()
	err := r.Err()
	if err != nil {
		return err
	}

	var allErrs []error
	infos, err := r.Infos()
	if err != nil {
		allErrs = append(allErrs, err)
	}

	errs := sets.NewString()
	first := true
	for _, info := range infos {
		mapping := info.ResourceMapping()
		describer, err := o.Describer(mapping)
		if err != nil {
			if errs.Has(err.Error()) {
				continue
			}
			allErrs = append(allErrs, err)
			errs.Insert(err.Error())
			continue
		}
		s, err := describer.Describe(info.Namespace, info.Name, *o.DescriberSettings)
		if err != nil {
			if errs.Has(err.Error()) {
				continue
			}
			allErrs = append(allErrs, err)
			errs.Insert(err.Error())
			continue
		}
		if first {
			first = false
			fmt.Fprint(o.Out, s)
		} else {
			fmt.Fprintf(o.Out, "\n\n%s", s)
		}
	}

	if len(infos) == 0 && len(allErrs) == 0 {
		// if we wrote no output, and had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}
