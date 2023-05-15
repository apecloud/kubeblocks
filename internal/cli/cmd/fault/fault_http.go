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
	"fmt"

	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/create"
)

var faultHTTPExample = templates.Examples(`
	# By default, the method of GET from port 80 is blocked.
	kbcli fault network http abort --duration=1m
	
	# Block the method of GET from port 4399.
	kbcli fault network http abort --port=4399 --duration=1m

	# Block the method of POST from port 4399.
	kbcli fault network http abort --port=4399 --method=POST --duration=1m

	# Delays post requests from port 4399.
	kbcli fault network http delay --port=4399 --method=POST --delay=15s
	
	# Replace the GET method sent from port 80 with the PUT method.
	kbcli fault network http replace --replace-method=PUT --duration=1m

	# Replace the GET method sent from port 80 with the PUT method, and replace the request body.
	kbcli fault network http replace --body="you are good luck" --replace-method=PUT --duration=2m

	# Replace the response content "you" from port 80.
	kbcli fault network http replace --target=Response --body=you --duration=30s
	
	# AAppend content to the body of the post request sent from port 4399, in JSON format.
	kbcli fault network http patch --method=POST --port=4399 --body="you are good luck" --type=JSON --duration=30s
`)

type HTTPChaosOptions struct {
	Target string `json:"target"`
	Port   int32  `json:"port"`
	Path   string `json:"path"`
	Method string `json:"method"`
	Code   int32  `json:"code,omitempty"`

	// abort command
	Abort bool `json:"abort,omitempty"`
	// delay command
	Delay string `json:"delay,omitempty"`

	// replace command
	ReplaceBody      []byte `json:"replaceBody,omitempty"`
	InputReplaceBody string `json:"-"`
	ReplacePath      string `json:"replacePath,omitempty"`
	ReplaceMethod    string `json:"replaceMethod,omitempty"`

	// patch command
	PatchBodyValue string `json:"patchBodyValue,omitempty"`
	PatchBodyType  string `json:"patchBodyType,omitempty"`

	FaultBaseOptions
}

func NewHTTPChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *HTTPChaosOptions {
	o := &HTTPChaosOptions{
		FaultBaseOptions: FaultBaseOptions{
			CreateOptions: create.CreateOptions{
				Factory:         f,
				IOStreams:       streams,
				CueTemplateName: CueTemplateHTTPChaos,
				GVR:             GetGVR(Group, Version, ResourceHTTPChaos),
			},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewHTTPChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http",
		Short: "Intercept HTTP requests and responses.",
	}
	cmd.AddCommand(
		NewAbortCmd(f, streams),
		NewHTTPDelayCmd(f, streams),
		NewReplaceCmd(f, streams),
		NewPatchCmd(f, streams),
	)
	return cmd
}

func NewAbortCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewHTTPChaosOptions(f, streams, "")

	cmd := o.NewCobraCommand(Abort, AbortShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().BoolVar(&o.Abort, "abort", true, `Indicates whether to inject the fault that interrupts the connection.`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewHTTPDelayCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewHTTPChaosOptions(f, streams, "")

	cmd := o.NewCobraCommand(HTTPDelay, HTTPDelayShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.Delay, "delay", "10s", `The time for delay.`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewReplaceCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewHTTPChaosOptions(f, streams, "")

	cmd := o.NewCobraCommand(Replace, ReplaceShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.InputReplaceBody, "body", "", `The content of the request body or response body to replace the failure.`)
	cmd.Flags().StringVar(&o.ReplacePath, "replace-path", "", `The URI path used to replace content.`)
	cmd.Flags().StringVar(&o.ReplaceMethod, "replace-method", "", `The replaced content of the HTTP request method.`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewPatchCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewHTTPChaosOptions(f, streams, "")

	cmd := o.NewCobraCommand(Patch, PatchShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.PatchBodyValue, "body", "", `The fault of the request body or response body with patch faults.`)
	cmd.Flags().StringVar(&o.PatchBodyType, "type", "", `The type of patch faults of the request body or response body. Currently, it only supports JSON.`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *HTTPChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultHTTPExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *HTTPChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	o.FaultBaseOptions.AddCommonFlag(cmd)

	cmd.Flags().StringVar(&o.Target, "target", "Request", `Specifies whether the target of fault injuection is Request or Response. The target-related fields should be configured at the same time.`)
	cmd.Flags().Int32Var(&o.Port, "port", 80, `The TCP port that the target service listens on.`)
	cmd.Flags().StringVar(&o.Path, "path", "*", `The URI path of the target request. Supports Matching wildcards.`)
	cmd.Flags().StringVar(&o.Method, "method", "GET", `The HTTP method of the target request method.For example: GET, POST, PUT, DELETE, HEAD, OPTIONS, PATCH.`)
	cmd.Flags().Int32Var(&o.Code, "code", 0, `The status code responded by target.`)
}

func (o *HTTPChaosOptions) Validate() error {
	if o.PatchBodyType != "" && o.PatchBodyType != "JSON" {
		return fmt.Errorf("the --type only supports JSON")
	}
	if o.PatchBodyValue != "" && o.PatchBodyType == "" {
		return fmt.Errorf("the --type is required when --body is specified")
	}
	if o.PatchBodyType != "" && o.PatchBodyValue == "" {
		return fmt.Errorf("the --body is required when --type is specified")
	}

	var msg interface{}
	if o.PatchBodyValue != "" && json.Unmarshal([]byte(o.PatchBodyValue), &msg) != nil {
		return fmt.Errorf("the --body is not a valid JSON")
	}

	if o.Target == "Request" && o.Code != 0 {
		return fmt.Errorf("the --code is only supported when --target is Response")
	}

	if ok, err := IsRegularMatch(o.Delay); !ok {
		return err
	}
	return o.BaseValidate()
}

func (o *HTTPChaosOptions) Complete() error {
	o.ReplaceBody = []byte(o.InputReplaceBody)
	return o.BaseComplete()
}

func (o *HTTPChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.HTTPChaos{}
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
