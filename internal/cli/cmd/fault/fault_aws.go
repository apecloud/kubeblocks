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

var faultAWSExample = templates.Examples(`
	# Stop a specified EC2 instance.
	kbcli fault aws stop --secret-name=cloud-key-secret --aws-region=cn-northwest-1 --ec2-instance=i-0a4986881adf30039 --duration=3m
	
	# Restart a specified EC2 instance.
	kbcli fault aws restart --secret-name=cloud-key-secret --aws-region=cn-northwest-1 --ec2-instance=i-0ff10a1487cf6bbac --duration=1m
	
	# Detach a specified volume from a specified EC2 instance.
	kbcli fault aws detach-volume --secret-name=cloud-key-secret --aws-region=cn-northwest-1 --ec2-instance=i-0df0732607d54dd8e --duration=1m --volume-id=vol-072f0940c28664f74 --device-name=/dev/xvdab
`)

type AWSChaosOptions struct {
	AwsRegion   string `json:"awsRegion"`
	Ec2Instance string `json:"ec2Instance"`
	VolumeID    string `json:"volumeID,omitempty"`
	DeviceName  string `json:"deviceName"`

	//NodeOptions

	create.CreateOptions `json:"-"`
}

func NewAWSOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *AWSChaosOptions {
	o := &AWSChaosOptions{
		CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: CueTemplateAWSChaos,
			GVR:             GetGVR(Group, Version, ResourceAWSChaos),
		},
		NodeOptions: NodeOptions{
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewAWSChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "aws",
		Short: "aws chaos.",
	}
	cmd.AddCommand(
		NewStopCmd(f, streams),
		NewRestartCmd(f, streams),
		NewDetachVolumeCmd(f, streams),
	)
	return cmd
}

func NewStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAWSOptions(f, streams, string(v1alpha1.Ec2Stop))
	cmd := o.NewCobraCommand(Stop, StopShort)

	o.AddCommonFlag(cmd)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAWSOptions(f, streams, string(v1alpha1.Ec2Restart))
	cmd := o.NewCobraCommand(Restart, RestartShort)

	o.AddCommonFlag(cmd)

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewDetachVolumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewAWSOptions(f, streams, string(v1alpha1.DetachVolume))
	cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringVar(&o.VolumeID, "volume-id", "", "The volume id of the ec2.")
	cmd.Flags().StringVar(&o.DeviceName, "device-name", "", "The device name of the volume.")

	util.CheckErr(cmd.MarkFlagRequired("volume-id"))
	util.CheckErr(cmd.MarkFlagRequired("device-name"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *AWSChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultAWSExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *AWSChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.SecretName, "secret-name", "", "The name of the Kubernetes Secret that stores the AWS authentication information.")
	cmd.Flags().StringVar(&o.AwsRegion, "region", "", "The region of the aws.")
	cmd.Flags().StringVar(&o.Ec2Instance, "instance", "", "The instance id of the ec2.")
	cmd.Flags().StringVar(&o.Duration, "duration", "30s", "Supported formats of the duration are: ms / s / m / h.")

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged

	util.CheckErr(cmd.MarkFlagRequired("secret-name"))
	util.CheckErr(cmd.MarkFlagRequired("region"))
	util.CheckErr(cmd.MarkFlagRequired("instance"))

	printer.AddOutputFlagForCreate(cmd, &o.Format)
}

func (o *AWSChaosOptions) Validate() error {
	if ok, err := IsRegularMatch(o.Duration); !ok {
		return err
	}
	return nil
}

func (o *AWSChaosOptions) Complete() error {
	return nil
}

func (o *AWSChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.AWSChaos{}
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
