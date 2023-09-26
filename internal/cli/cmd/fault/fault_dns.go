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
	"github.com/chaos-mesh/chaos-mesh/api/v1alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var faultDNSExample = templates.Examples(`
	// Inject DNS faults into all pods under the default namespace, so that any IP is returned when accessing the bing.com domain name.
	kbcli fault dns random --patterns=bing.com --duration=1m

	// Inject DNS faults into all pods under the default namespace, so that error is returned when accessing the bing.com domain name.
	kbcli fault dns error --patterns=bing.com --duration=1m
`)

type DNSChaosOptions struct {
	Patterns []string `json:"patterns"`

	FaultBaseOptions
}

func NewDNSChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *DNSChaosOptions {
	o := &DNSChaosOptions{
		FaultBaseOptions: FaultBaseOptions{
			CreateOptions: create.CreateOptions{
				Factory:         f,
				IOStreams:       streams,
				CueTemplateName: CueTemplateDNSChaos,
				GVR:             GetGVR(Group, Version, ResourceDNSChaos),
			},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewDNSChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "Inject faults into DNS server.",
	}
	cmd.AddCommand(
		NewRandomCmd(f, streams),
		NewErrorCmd(f, streams),
	)
	return cmd
}

func NewRandomCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDNSChaosOptions(f, streams, string(v1alpha1.RandomAction))
	cmd := o.NewCobraCommand(Random, RandomShort)

	o.AddCommonFlag(cmd)
	util.CheckErr(cmd.MarkFlagRequired("patterns"))

	return cmd
}

func NewErrorCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDNSChaosOptions(f, streams, string(v1alpha1.ErrorAction))
	cmd := o.NewCobraCommand(Error, ErrorShort)

	o.AddCommonFlag(cmd)
	util.CheckErr(cmd.MarkFlagRequired("patterns"))

	return cmd
}

func (o *DNSChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultDNSExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *DNSChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	o.FaultBaseOptions.AddCommonFlag(cmd)

	cmd.Flags().StringArrayVar(&o.Patterns, "patterns", nil, `Select the domain name template that matching the failure behavior & supporting placeholders ? and wildcards *.`)

	// register flag completion func
	registerFlagCompletionFunc(cmd, o.Factory)
}

func (o *DNSChaosOptions) Validate() error {
	return o.BaseValidate()
}

func (o *DNSChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *DNSChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.DNSChaos{}
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
