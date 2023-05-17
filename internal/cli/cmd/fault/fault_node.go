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
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	cp "github.com/apecloud/kubeblocks/internal/cli/cloudprovider"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var faultNodeExample = templates.Examples(`
	# Stop a specified EC2 instance.
	kbcli fault node stop aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0a4986881adf30039 --duration=3m
	
	# Stop a specified EC2 instance.
	kbcli fault node stop -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0a4986881adf30039 --duration=3m

	# Restart a specified EC2 instance.
	kbcli fault node restart aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0ff10a1487cf6bbac --duration=1m

	# Detach a specified volume from a specified EC2 instance.
	kbcli fault node detach-volume aws --secret-name=cloud-key-secret --region=cn-northwest-1 --instance=i-0df0732607d54dd8e --duration=1m --volume-id=vol-072f0940c28664f74 --device-name=/dev/xvdab
	
	# Stop a specified GCK instance.
	kbcli fault node stop gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-45rl --secret-name=cloud-key-secret
		
	# Stop a specified GCK instance.
	kbcli fault node stop -c=gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-45rl --secret-name=cloud-key-secret

	# Restart a specified GCK instance.
	kbcli fault node restart gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-d9nd --secret-name=cloud-key-secret

	# Detach a specified volume from a specified GCK instance.
	kbcli fault node detach-volume gcp --region=us-central1-c --project=apecloud-platform-engineering --instance=gke-hyqtest-default-pool-2fe51a08-d9nd --secret-name=cloud-key-secret --device-name=/dev/sdb
`)

type NodeChaoOptions struct {
	Kind string `json:"kind"`

	Action string `json:"action"`

	CloudProvider string `json:"-"`

	SecretName string `json:"secretName"`

	Region string `json:"region"`

	Instance string `json:"instance"`

	VolumeID string `json:"volumeID"`

	DeviceName string `json:"deviceName,omitempty"`

	Project string `json:"project"`

	Duration string `json:"duration"`

	create.CreateOptions `json:"-"`
}

func NewNodeOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *NodeChaoOptions {
	o := &NodeChaoOptions{
		CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: CueTemplateNodeChaos,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewNodeChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Node chaos.",
	}

	cmd.AddCommand(
		NewStopCmd(f, streams),
		NewRestartCmd(f, streams),
		NewDetachVolumeCmd(f, streams),
	)
	return cmd
}

func NewStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeOptions(f, streams)
	cmd := o.NewCobraCommand(Stop, StopShort)

	o.AddCommonFlag(cmd, f)
	return cmd
}

func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeOptions(f, streams)
	cmd := o.NewCobraCommand(Restart, RestartShort)

	o.AddCommonFlag(cmd, f)
	return cmd
}

func NewDetachVolumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeOptions(f, streams)
	cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)

	o.AddCommonFlag(cmd, f)
	cmd.Flags().StringVar(&o.VolumeID, "volume-id", "", "The volume id of the ec2.")
	cmd.Flags().StringVar(&o.DeviceName, "device-name", "", "The device name of the volume.")

	util.CheckErr(cmd.MarkFlagRequired("device-name"))
	return cmd
}

func (o *NodeChaoOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultNodeExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Complete(use))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *NodeChaoOptions) AddCommonFlag(cmd *cobra.Command, f cmdutil.Factory) {
	cmd.Flags().StringVarP(&o.CloudProvider, "cloud-provider", "c", "", fmt.Sprintf("Cloud provider type, one of %v", supportedCloudProviders))
	cmd.Flags().StringVar(&o.SecretName, "secret-name", "", "The name of the Kubernetes Secret that stores the AWS authentication information.")
	cmd.Flags().StringVar(&o.Region, "region", "", "The region of the aws.")
	cmd.Flags().StringVar(&o.Instance, "instance", "", "The instance id of the ec2.")
	cmd.Flags().StringVar(&o.Duration, "duration", "30s", "Supported formats of the duration are: ms / s / m / h.")
	cmd.Flags().StringVar(&o.Project, "project", "", "The ID of the GCP project.Only available when cloud-provider=gcp.")

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged
	printer.AddOutputFlagForCreate(cmd, &o.Format)

	util.CheckErr(cmd.MarkFlagRequired("secret-name"))
	util.CheckErr(cmd.MarkFlagRequired("region"))
	util.CheckErr(cmd.MarkFlagRequired("instance"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)
}

func (o *NodeChaoOptions) Validate() error {
	if ok, err := IsRegularMatch(o.Duration); !ok {
		return err
	}

	if o.CloudProvider == "" {
		return fmt.Errorf("cloud provider is required" + fmt.Sprintf("Cloud provider type, one of %v", supportedCloudProviders))
	}

	if o.CloudProvider == cp.AWS && o.Project != "" {
		return fmt.Errorf("you must not use --project to specify the projectID when cloud provider is aws")
	}

	if o.CloudProvider == cp.GCP && o.Project == "" {
		return fmt.Errorf("you must use --project to specify the projectID when cloud provider is gcp")
	}

	if o.CloudProvider == cp.AWS && o.Action == DetachVolume && o.VolumeID == "" {
		return fmt.Errorf("you must use --volume-id to specify the volumeID when cloud provider is aws")
	}

	if o.CloudProvider == cp.GCP && o.VolumeID != "" {
		return fmt.Errorf("you must not use --volume-id to specify the volumeID when cloud provider is gcp")
	}
	return nil
}

func (o *NodeChaoOptions) Complete(action string) error {
	if len(o.Args) > 0 {
		o.CloudProvider = o.Args[0]
	}
	if o.CloudProvider == cp.AWS {
		o.GVR = GetGVR(Group, Version, ResourceAWSChaos)
		o.Kind = KindAWSChaos
		switch action {
		case Stop:
			o.Action = string(v1alpha1.Ec2Stop)
		case Restart:
			o.Action = string(v1alpha1.Ec2Restart)
		case DetachVolume:
			o.Action = string(v1alpha1.DetachVolume)
		}
	} else if o.CloudProvider == cp.GCP {
		o.GVR = GetGVR(Group, Version, ResourceGCPChaos)
		o.Kind = KindGCPChaos
		switch action {
		case Stop:
			o.Action = string(v1alpha1.NodeStop)
		case Restart:
			o.Action = string(v1alpha1.NodeReset)
		case DetachVolume:
			o.Action = string(v1alpha1.DiskLoss)
		}
	}
	return nil
}

func (o *NodeChaoOptions) PreCreate(obj *unstructured.Unstructured) error {
	var c v1alpha1.InnerObject

	if o.CloudProvider == cp.AWS {
		c = &v1alpha1.AWSChaos{}
	} else if o.CloudProvider == cp.GCP {
		c = &v1alpha1.GCPChaos{}
	}

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
