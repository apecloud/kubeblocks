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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	// Options a command options object which extends BaseOptions
	Options interface{}

	// CueTemplateName cue template file name
	CueTemplateName string

	// ResourceName k8s resource name
	ResourceName string

	// Factory
	Factory cmdutil.Factory

	// ValidateFunc custom validate func
	ValidateFunc func() error

	// OptionsConvertFunc do custom options conversion
	OptionsConvertFunc func() error
}

// BaseOptions the options of creation command should inherit baseOptions
type BaseOptions struct {
	// Namespace k8s namespace
	Namespace string `json:"namespace"`
	// Name k8s resource metadata.name
	Name string `json:"name"`

	client dynamic.Interface
	genericclioptions.IOStreams
}

// Complete complete options struct default variables
func (o *BaseOptions) Complete(f cmdutil.Factory, args []string) error {
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		o.Name = args[0]
	}

	// used to fetch the resource
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return err
	}

	o.client = client

	return nil
}

// Run execute command. the options of parameter contain the command flags and args.
func (o *BaseOptions) Run(inputs Inputs, args []string) error {
	var (
		cueValue        cue.Value
		err             error
		unstructuredObj *unstructured.Unstructured
		optionsByte     []byte
	)
	// complete options variables
	if err = o.Complete(inputs.Factory, args); err != nil {
		return err
	}

	// do options custom convert
	if inputs.OptionsConvertFunc != nil {
		if err = inputs.OptionsConvertFunc(); err != nil {
			return err
		}
	}

	// do options validate
	if inputs.ValidateFunc != nil {
		if err = inputs.ValidateFunc(); err != nil {
			return err
		}
	}

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

	// create k8s resource
	gvr := schema.GroupVersionResource{Group: types.Group, Version: types.Version, Resource: inputs.ResourceName}
	if _, err = o.client.Resource(gvr).Namespace(o.Namespace).Create(context.TODO(), unstructuredObj, metav1.CreateOptions{}); err != nil {
		return err
	}
	kind, _ := getResourceKind(cueValue)
	fmt.Printf("%s %s created\n", kind, o.Name)
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

// getResourceKind
func getResourceKind(cueValue cue.Value) (string, error) {
	return cueValue.LookupPath(cue.ParsePath("content.kind")).String()
}
