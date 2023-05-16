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
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var faultGCPExample = templates.Examples(`
`)

type GCPChaosOptions struct {
	Project     string   `json:"project"`
	Zone        string   `json:"zone"`
	Instance    string   `json:"instance"`
	DeviceNames []string `json:"deviceNames,omitempty"`

	NodeOptions
	create.CreateOptions `json:"-"`
}

func NewGCPOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *GCPChaosOptions {
	o := &GCPChaosOptions{
		CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: CueTemplateGCPChaos,
			GVR:             GetGVR(Group, Version, ResourceGCPChaos),
		},
		NodeOptions: NodeOptions{
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewGCPCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcp",
		Short: "gcp chaos.",
	}
	cmd.AddCommand(
		NewGCPStopCmd(f, streams),
		NewGCPRestartCmd(f, streams),
		NewGCpDetachVolumeCmd(f, streams),
	)
	return cmd
}

func NewGCPStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGCPOptions(f, streams, string(v1alpha1.NodeStop))
	cmd := o.NewCobraCommand(Stop, StopShort)

	o.AddCommonFlag(cmd)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewGCPRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGCPOptions(f, streams, string(v1alpha1.NodeReset))
	cmd := o.NewCobraCommand(Restart, RestartShort)

	o.AddCommonFlag(cmd)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewGCpDetachVolumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewGCPOptions(f, streams, string(v1alpha1.DiskLoss))
	cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringArrayVar(&o.DeviceNames, "device-name", nil, "The device name of the volumes.")
	util.CheckErr(cmd.MarkFlagRequired("device-name"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *GCPChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultGCPExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *GCPChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.SecretName, "secret-name", "", "The name of the Kubernetes Secret that stores GCP authentication information.")
	cmd.Flags().StringVar(&o.Zone, "region", "", "The region of GCP instance.")
	cmd.Flags().StringVar(&o.Instance, "instance", "", "The name of GCP instance.")
	cmd.Flags().StringVar(&o.Duration, "duration", "30s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringVar(&o.Project, "project", "", "The ID of the GCP project.")

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	util.CheckErr(cmd.MarkFlagRequired("secret-name"))
	util.CheckErr(cmd.MarkFlagRequired("region"))
	util.CheckErr(cmd.MarkFlagRequired("instance"))
	util.CheckErr(cmd.MarkFlagRequired("project"))

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *GCPChaosOptions) Validate() error {
	if ok, err := IsRegularMatch(o.Duration); !ok {
		return err
	}
	return nil
}

func (o *GCPChaosOptions) Complete() error {
	return nil
}

func (o *GCPChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.GCPChaos{}
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
