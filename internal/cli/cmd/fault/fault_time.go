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

var faultTimeExample = templates.Examples(`
	# Affects the first container in default namespace's all pods.Shifts the clock back five seconds.
	kbcli fault time --time-offset=-5s
	
	# Affects the first container in default namespace's all pods.
	kbcli fault time --time-offset=-5m5s
	
	# Affects the first container in mycluster-mysql-0 pod. Shifts the clock forward five seconds.
	kbcli fault time mycluster-mysql-0 --time-offset=+5s50ms

	# Affects the mysql container in mycluster-mysql-0 pod. Shifts the clock forward five seconds.
	kbcli fault time mycluster-mysql-0 --time-offset=+5s -c=mysql
	
	# The clock that specifies the effect of time offset is CLOCK_REALTIME.
	kbcli fault time mycluster-mysql-0 --time-offset=+5s --clock-id=CLOCK_REALTIME -c=mysql
`)

type TimeChaosOptions struct {
	TimeOffset string `json:"timeOffset"`

	ClockIds []string `json:"clockIds,omitempty"`

	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions
}

func NewTimeChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *TimeChaosOptions {
	o := &TimeChaosOptions{
		FaultBaseOptions: FaultBaseOptions{
			CreateOptions: create.CreateOptions{
				Factory:         f,
				IOStreams:       streams,
				CueTemplateName: CueTemplateTimeChaos,
				GVR:             GetGVR(Group, Version, ResourceTimeChaos),
			},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewTimeChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewTimeChaosOptions(f, streams, "")
	cmd := o.NewCobraCommand(Time, TimeShort)

	o.AddCommonFlag(cmd, f)
	return cmd
}

func (o *TimeChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultTimeExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *TimeChaosOptions) AddCommonFlag(cmd *cobra.Command, f cmdutil.Factory) {
	o.FaultBaseOptions.AddCommonFlag(cmd)

	cmd.Flags().StringVar(&o.TimeOffset, "time-offset", "", "Specifies the length of the time offset. For example: -5s, -10m100ns.")
	cmd.Flags().StringArrayVar(&o.ClockIds, "clock-id", nil, `Specifies the clock on which the time offset acts.If it's empty, it will be set to ['CLOCK_REALTIME'].See clock_gettime [https://man7.org/linux/man-pages/man2/clock_gettime.2.html] document for details.`)
	cmd.Flags().StringArrayVarP(&o.ContainerNames, "container", "c", nil, `Specifies the injected container name. For example: mysql. If it's empty, the first container will be injected.`)

	util.CheckErr(cmd.MarkFlagRequired("time-offset"))

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)
}

func (o *TimeChaosOptions) Validate() error {
	return o.BaseValidate()
}

func (o *TimeChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *TimeChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.TimeChaos{}
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
