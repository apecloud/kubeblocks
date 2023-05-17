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

	"github.com/apecloud/kubeblocks/internal/cli/create"
)

var faultStressExample = templates.Examples(`
	# Affects the first container in default namespace's all pods.Making CPU load up to 50%, and the memory up to 100MB. 
	kbcli fault stress --cpu-worker=2 --cpu-load=50 --memory-worker=1 --memory-size=100Mi

	# Affects the first container in mycluster-mysql-0 pod. Making the CPU load up to 50%, and the memory up to 500MB.
	kbcli fault stress mycluster-mysql-0 --cpu-worke=2 --cpu-load=50
	
	# Affects the mysql container in mycluster-mysql-0 pod. Making the memory up to 500MB.
	kbcli fault stress mycluster-mysql-0 --memory-worker=2 --memory-size=500Mi  -c=mysql
`)

type CPU struct {
	Workers int `json:"workers"`
	Load    int `json:"load"`
}

type Memory struct {
	Workers int    `json:"workers"`
	Size    string `json:"size"`
}

type Stressors struct {
	CPU    `json:"cpu"`
	Memory `json:"memory"`
}

type StressChaosOptions struct {
	Stressors      `json:"stressors"`
	ContainerNames []string `json:"containerNames,omitempty"`

	FaultBaseOptions
}

func NewStressChaosOptions(f cmdutil.Factory, streams genericclioptions.IOStreams, action string) *StressChaosOptions {
	o := &StressChaosOptions{
		FaultBaseOptions: FaultBaseOptions{
			CreateOptions: create.CreateOptions{
				Factory:         f,
				IOStreams:       streams,
				CueTemplateName: CueTemplateStressChaos,
				GVR:             GetGVR(Group, Version, ResourceStressChaos),
			},
			Action: action,
		},
	}
	o.CreateOptions.PreCreate = o.PreCreate
	o.CreateOptions.Options = o
	return o
}

func NewStressChaosCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewStressChaosOptions(f, streams, "")
	cmd := o.NewCobraCommand(Stress, StressShort)

	o.AddCommonFlag(cmd, f)
	return cmd
}

func (o *StressChaosOptions) NewCobraCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Short:   short,
		Example: faultStressExample,
		Run: func(cmd *cobra.Command, args []string) {
			o.Args = args
			cmdutil.CheckErr(o.CreateOptions.Complete())
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Complete())
			cmdutil.CheckErr(o.Run())
		},
	}
}

func (o *StressChaosOptions) AddCommonFlag(cmd *cobra.Command, f cmdutil.Factory) {
	o.FaultBaseOptions.AddCommonFlag(cmd)

	cmd.Flags().IntVar(&o.CPU.Workers, "cpu-worker", 0, `Specifies the number of threads that exert CPU pressure.`)
	cmd.Flags().IntVar(&o.CPU.Load, "cpu-load", 0, `Specifies the percentage of CPU occupied. 0 means no extra load added, 100 means full load. The total load is workers * load.`)
	cmd.Flags().IntVar(&o.Memory.Workers, "memory-worker", 0, `Specifies the number of threads that apply memory pressure.`)
	cmd.Flags().StringVar(&o.Memory.Size, "memory-size", "", `Specify the size of the allocated memory or the percentage of the total memory, and the sum of the allocated memory is size. For example:256MB or 25%`)
	cmd.Flags().StringArrayVarP(&o.ContainerNames, "container", "c", nil, "The name of the container, such as mysql, prometheus.If it's empty, the first container will be injected.")

	// register flag completion func
	registerFlagCompletionFunc(cmd, f)
}

func (o *StressChaosOptions) Validate() error {
	if o.Memory.Workers == 0 && o.CPU.Workers == 0 {
		return fmt.Errorf("the CPU or Memory workers must have at least one greater than 0, Use --cpu-workers or --memory-workers to specify")
	}

	return o.BaseValidate()
}

func (o *StressChaosOptions) Complete() error {
	return o.BaseComplete()
}

func (o *StressChaosOptions) PreCreate(obj *unstructured.Unstructured) error {
	c := &v1alpha1.StressChaos{}
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
