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
	kbcli fault node stop node1 -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --duration=3m

	# Stop two specified EC2 instances.
	kbcli fault node stop node1 node2 -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --duration=3m

	# Restart two specified EC2 instances.
	kbcli fault node restart node1 node2 -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --duration=3m

	# Detach two specified volume from two specified EC2 instances.
	kbcli fault node detach-volume node1 node2 -c=aws --secret-name=cloud-key-secret --region=cn-northwest-1 --duration=1m --volume-id=v1,v2 --device-name=/d1,/d2

	# Stop two specified GCK instances.
	kbcli fault node stop node1 node2 -c=gcp --region=us-central1-c --project=apecloud-platform-engineering --secret-name=cloud-key-secret	

	# Restart two specified GCK instances.
	kbcli fault node restart node1 node2 -c=gcp --region=us-central1-c --project=apecloud-platform-engineering --secret-name=cloud-key-secret

	# Detach two specified volume from two specified GCK instances.
	kbcli fault node detach-volume node1 node2 -c=gcp --region=us-central1-c --project=apecloud-platform-engineering --secret-name=cloud-key-secret --device-name=/d1,/d2
`)

type NodeChaoOptions struct {
	Kind string `json:"kind"`

	Action string `json:"action"`

	CloudProvider string `json:"-"`

	SecretName string `json:"secretName"`

	Region string `json:"region"`

	Instance string `json:"instance"`

	VolumeID  string   `json:"volumeID"`
	VolumeIDs []string `json:"-"`

	DeviceName  string   `json:"deviceName,omitempty"`
	DeviceNames []string `json:"-"`

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

	o.AddCommonFlag(cmd)
	return cmd
}

func NewRestartCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeOptions(f, streams)
	cmd := o.NewCobraCommand(Restart, RestartShort)

	o.AddCommonFlag(cmd)
	return cmd
}

func NewDetachVolumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNodeOptions(f, streams)
	cmd := o.NewCobraCommand(DetachVolume, DetachVolumeShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringSliceVar(&o.VolumeIDs, "volume-id", nil, "The volume ids of the ec2. Only available when cloud-provider=aws.")
	cmd.Flags().StringSliceVar(&o.DeviceNames, "device-name", nil, "The device name of the volume.")

	util.CheckErr(cmd.MarkFlagRequired("device-name"))
	return cmd
}

func (o *NodeChaoOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultNodeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Execute(use, args))
		},
	}
}

func (o *NodeChaoOptions) Execute(action string, args []string) error {
	o.Args = args
	if err := o.CreateOptions.Complete(); err != nil {
		return err
	}
	if err := o.Complete(action); err != nil {
		return err
	}
	if err := o.Validate(); err != nil {
		return err
	}

	for idx, arg := range o.Args {
		o.Instance = arg
		if o.DeviceNames != nil {
			o.DeviceName = o.DeviceNames[idx]
		}
		if o.VolumeIDs != nil {
			o.VolumeID = o.VolumeIDs[idx]
		}
		if err := o.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (o *NodeChaoOptions) AddCommonFlag(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.CloudProvider, "cloud-provider", "c", "", fmt.Sprintf("Cloud provider type, one of %v", supportedCloudProviders))
	cmd.Flags().StringVar(&o.SecretName, "secret-name", "", "The name of the Kubernetes Secret that stores the kubernetes cluster authentication information.")
	cmd.Flags().StringVar(&o.Region, "region", "", "The region of the node.")
	cmd.Flags().StringVar(&o.Project, "project", "", "The name of the GCP project. Only available when cloud-provider=gcp.")
	cmd.Flags().StringVar(&o.Duration, "duration", "30s", "Supported formats of the duration are: ms / s / m / h.")

	cmd.Flags().StringVar(&o.DryRun, "dry-run", "none", `Must be "client", or "server". If client strategy, only print the object that would be sent, without sending it. If server strategy, submit server-side request without persisting the resource.`)
	cmd.Flags().Lookup("dry-run").NoOptDefVal = Unchanged
	printer.AddOutputFlagForCreate(cmd, &o.Format)

	util.CheckErr(cmd.MarkFlagRequired("cloud-provider"))
	util.CheckErr(cmd.MarkFlagRequired("secret-name"))
	util.CheckErr(cmd.MarkFlagRequired("region"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, o.Factory)
}

func (o *NodeChaoOptions) Validate() error {
	if ok, err := IsRegularMatch(o.Duration); !ok {
		return err
	}

	if len(o.Args) == 0 {
		return fmt.Errorf("node instance is required")
	}

	switch o.CloudProvider {
	case cp.AWS:
		if o.Project != "" {
			return fmt.Errorf("--project is not supported when cloud provider is aws")
		}
		if o.Action == DetachVolume && o.VolumeIDs == nil {
			return fmt.Errorf("--volume-id is required when cloud provider is aws")
		}
		if o.Action == DetachVolume && len(o.DeviceNames) != len(o.VolumeIDs) {
			return fmt.Errorf("the number of volume-id must be equal to the number of device-name")
		}
	case cp.GCP:
		if o.Project == "" {
			return fmt.Errorf("--project is required when cloud provider is gcp")
		}
		if o.VolumeIDs != nil {
			return fmt.Errorf(" --volume-id is not supported when cloud provider is gcp")
		}
	default:
		return fmt.Errorf("cloud provider type, one of %v", supportedCloudProviders)
	}

	if o.DeviceNames != nil && len(o.Args) != len(o.DeviceNames) {
		return fmt.Errorf("the number of device-name must be equal to the number of node")
	}
	return nil
}

func (o *NodeChaoOptions) Complete(action string) error {
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
