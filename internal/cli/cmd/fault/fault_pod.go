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

var faultPodExample = templates.Examples(`
	# kill all pods in default namespace
	kbcli fault pod kill
	
	# kill any pod in default namespace
	kbcli fault pod kill --mode=one

	# kill two pods in default namespace
	kbcli fault pod kill --mode=fixed --value=2

	# kill 50% pods in default namespace
	kbcli fault pod kill --mode=percentage --value=50

	# kill mysql-cluster-mysql-0 pod in default namespace
	kbcli fault pod kill mysql-cluster-mysql-0

	# kill all pods in default namespace
	kbcli fault pod kill --ns-fault="default"

	# --label is required to specify the pods that need to be killed. 
	kbcli fault pod kill --label statefulset.kubernetes.io/pod-name=mysql-cluster-mysql-2

	# kill pod under the specified node.
	kbcli fault pod kill --node=minikube-m02
	
	# kill pod under the specified node-label.
	kbcli fault pod kill --node-label=kubernetes.io/arch=arm64

	# Allow the experiment to last for one minute.
	kbcli fault pod failure --duration=1m

	# kill container in pod
	kbcli fault pod kill-container mysql-cluster-mysql-0 --container=mysql
`)

type PodChaosOptions struct {
	// GracePeriod waiting time, after which fault injection is performed
	GracePeriod    int64    `json:"gracePeriod"`
	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions
}

func NewPodChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *PodChaosOptions {
	o := &PodChaosOptions{
		FaultBaseOptions: FaultBaseOptions{
			CreateOptions: create.CreateOptions{
				Factory:         f,
				IOStreams:       streams,
				CueTemplateName: CueTemplatePodChaos,
				GVR:             GetGVR(Group, Version, ResourcePodChaos),
			},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewPodChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod",
		Short: "pod chaos.",
	}
	cmd.AddCommand(
		NewPodKillCmd(f, streams),
		NewPodFailureCmd(f, streams),
		NewContainerKillCmd(f, streams),
	)
	return cmd
}

func NewPodKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPodChaosOptions(f, streams, string(v1alpha1.PodKillAction))
	cmd := o.NewCobraCommand(Kill, KillShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().Int64VarP(&o.GracePeriod, "grace-period", "g", 0, "Grace period represents the duration in seconds before the pod should be killed")

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewPodFailureCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPodChaosOptions(f, streams, string(v1alpha1.PodFailureAction))
	cmd := o.NewCobraCommand(Failure, FailureShort)

	o.AddCommonFlag(cmd)
	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func NewContainerKillCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewPodChaosOptions(f, streams, string(v1alpha1.ContainerKillAction))
	cmd := o.NewCobraCommand(KillContainer, KillContainerShort)

	o.AddCommonFlag(cmd)
	cmd.Flags().StringArrayVarP(&o.ContainerNames, "container", "c", nil, "the name of the container you want to kill, such as mysql, prometheus.")

	util.CheckErr(cmd.MarkFlagRequired("container"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)

	return cmd
}

func (o *PodChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultPodExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *PodChaosOptions) AddCommonFlag(cmd *cobra.Command) {
	o.FaultBaseOptions.AddCommonFlag(cmd)
}

func (o *PodChaosOptions) Validate() error {
	return o.BaseValidate()
}

func (o *PodChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *PodChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.PodChaos{}
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
