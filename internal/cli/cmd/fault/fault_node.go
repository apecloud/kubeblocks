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
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

type NodeChaoOptions struct {
	Action     string `json:"action"`
	SecretName string `json:"secretName"`
	Duration   string `json:"duration"`

	AwsRegion   string `json:"awsRegion"`
	Ec2Instance string `json:"ec2Instance"`
	VolumeID    string `json:"volumeID,omitempty"`
	DeviceName  string `json:"deviceName"`

	create.CreateOptions `json:"-"`
}

func NewNodeOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *NodeChaoOptions {
	o := &NodeChaoOptions{
		CreateOptions: create.CreateOptions{
			Factory:         f,
			IOStreams:       streams,
			CueTemplateName: CueTemplateAWSChaos,
			//GVR:             GetGVR(Group, Version, ResourceAWSChaos),
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewNodeChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "node chaos.",
	}
	//cmd.AddCommand(
	//	NewAWSChaosCmd(f, streams),
	//	NewGCPCmd(f, streams),
	//)
	cmd.AddCommand(
		NewStopCmd(f, streams),
		NewRestartCmd(f, streams),
		NewDetachVolumeCmd(f, streams),
	)

	return cmd
}

//func NewStopCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
//	o := NewAWSOptions(f, streams)
//	cmd := o.NewCobraCommand(Stop, StopShort)
//
//	o.AddCommonFlag(cmd)
//
//	// register flag completion func
//	registerFlagCompletionFunc(cmd, f)
//
//	return cmd
//}
