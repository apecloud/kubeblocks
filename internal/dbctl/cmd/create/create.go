/*
Copyright ApeCloud Inc.

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

package create

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	cuejson "cuelang.org/go/encoding/json"
	"github.com/leaanthony/debme"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sapitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/dbctl/types"
)

var (
	//go:embed template/*
	cueTemplate embed.FS
)

type Inputs struct {
	// Use cobra command use
	Use string

	// Short is the short description shown in the 'help' output.
	Short string

	// Example is examples of how to use the command.
	Example string

	// BaseOptionsObj
	BaseOptionsObj *BaseOptions

	// Options a command options object which extends BaseOptions
	Options interface{}

	// CueTemplateName cue template file name
	CueTemplateName string

	// ResourceName k8s resource name
	ResourceName string

	// Group of API, default is dbaas
	Group string

	// Group of Version, default is v1alpha1
	Version string

	// Factory
	Factory cmdutil.Factory

	// ValidateFunc optional, custom validate func
	Validate func() error

	// Complete optional, do custom complete options
	Complete func() error

	BuildFlags func(*cobra.Command)

	// PreCreate optional, make changes on yaml before create
	PreCreate func(*unstructured.Unstructured) error
}

// BaseOptions the options of creation command should inherit baseOptions
type BaseOptions struct {
	// Namespace k8s namespace
	Namespace string `json:"namespace"`

	// Name Resource name of the command line operation
	Name string `json:"name"`

	Client dynamic.Interface `json:"-"`
	genericclioptions.IOStreams
}

// BuildCommand build create command
func BuildCommand(inputs Inputs) *cobra.Command {
	cmd := &cobra.Command{
		Use:     inputs.Use,
		Short:   inputs.Short,
		Example: inputs.Example,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(inputs.BaseOptionsObj.Complete(inputs, args))
			cmdutil.CheckErr(inputs.BaseOptionsObj.Validate(inputs))
			cmdutil.CheckErr(inputs.BaseOptionsObj.Run(inputs))
		},
	}
	if inputs.BuildFlags != nil {
		inputs.BuildFlags(cmd)
	}
	return cmd
}

func (o *BaseOptions) Complete(inputs Inputs, args []string) error {
	var err error
	if o.Namespace, _, err = inputs.Factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	if o.Client, err = inputs.Factory.DynamicClient(); err != nil {
		return err
	}

	// do custom options complete
	if inputs.Complete != nil {
		if err = inputs.Complete(); err != nil {
			return err
		}
	}
	return nil
}

func (o *BaseOptions) Validate(inputs Inputs) error {
	// do options validate
	if inputs.Validate != nil {
		if err := inputs.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *BaseOptions) Run(inputs Inputs) error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(inputs.Options); err != nil {
		return err
	}

	if cueValue, err = newCueValue(inputs.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = covertContentToUnstructured(cueValue); err != nil {
		return err
	}

	if inputs.PreCreate != nil {
		if err = inputs.PreCreate(unstructuredObj); err != nil {
			return err
		}
	}
	group := inputs.Group
	if len(group) == 0 {
		group = types.Group
	}

	version := inputs.Version
	if len(version) == 0 {
		version = types.Version
	}
	// create k8s resource
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: inputs.ResourceName}
	if unstructuredObj, err = o.Client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
		return err
	}
	fmt.Fprintf(o.Out, "%s %s created\n", unstructuredObj.GetKind(), unstructuredObj.GetName())
	return nil
}

// RunAsApply execute command. the options of parameter contain the command flags and args.
// if the resource exists, run as "kubectl apply".
func (o *BaseOptions) RunAsApply(inputs Inputs) error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)

	if optionsByte, err = json.Marshal(inputs.Options); err != nil {
		return err
	}

	if cueValue, err = newCueValue(inputs.CueTemplateName); err != nil {
		return err
	}

	if cueValue, err = fillOptions(cueValue, optionsByte); err != nil {
		return err
	}

	if unstructuredObj, err = covertContentToUnstructured(cueValue); err != nil {
		return err
	}

	group := inputs.Group
	if len(group) == 0 {
		group = types.Group
	}

	version := inputs.Version
	if len(version) == 0 {
		version = types.Version
	}
	// create k8s resource
	gvr := schema.GroupVersionResource{Group: group, Version: version, Resource: inputs.ResourceName}
	objectName, _, err := unstructured.NestedString(unstructuredObj.Object, "metadata", "name")
	if err != nil {
		return err
	}
	objectByte, err := json.Marshal(unstructuredObj)
	if err != nil {
		return err
	}
	if _, err := o.Client.Resource(gvr).Namespace(o.Namespace).Patch(
		context.TODO(), objectName, k8sapitypes.MergePatchType,
		objectByte, metav1.PatchOptions{}); err != nil {

		// create object if not found
		if errors.IsNotFound(err) {
			if _, err = o.Client.Resource(gvr).Namespace(o.Namespace).Create(
				context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// NewCueValue covert cue template  to cue Value which holds any value like Boolean,Struct,String and more cue type.
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

// covertContentToUnstructured get content object in cue template file and covert it to Unstructured
func covertContentToUnstructured(cueValue cue.Value) (*unstructured.Unstructured, error) {
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
