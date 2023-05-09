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

package sync2foxlake

import (
	"context"
	"encoding/json"

	"github.com/docker/cli/cli"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/patch"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/sync2foxlakeapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	pauseExample = templates.Examples(`
		# pause a sync2foxlake task named mytask
		kbcli sync2foxlake pause mytask
	`)
	resumeExample = templates.Examples(`
		# resume a sync2foxlake task named mytask
		kbcli sync2foxlake resume mytask	
	`)
)

type Sync2FoxLakePatchOptions struct {
	Factory cmdutil.Factory
	Dynamic dynamic.Interface

	Namespace string
	Task      *v1alpha1.Sync2FoxLakeTask

	*patch.Options
}

func (o *Sync2FoxLakePatchOptions) complete(args []string, patchfunc func(*v1alpha1.Sync2FoxLakeTask)) error {
	var err error
	if len(args) > 0 {
		o.Names = args
	}
	o.Dynamic, err = o.Factory.DynamicClient()
	if err != nil {
		return err
	}
	o.Namespace, _, err = o.Factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.Task = &v1alpha1.Sync2FoxLakeTask{}
	obj, err := o.Dynamic.Resource(types.Sync2FoxLakeTaskGVR()).Namespace(o.Namespace).Get(context.Background(), o.Names[0], metav1.GetOptions{}, "")
	if err != nil {
		return err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, o.Task)
	if err != nil {
		return err
	}

	patchfunc(o.Task)
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o.Task)
	if err != nil {
		return err
	}
	patchBytes, err := json.Marshal(unstructuredObj)
	if err != nil {
		return err
	}
	o.Patch = string(patchBytes)

	return nil

}

func NewSync2FoxLakePauseCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &Sync2FoxLakePatchOptions{
		Factory: f,
		Options: patch.NewOptions(f, streams, types.Sync2FoxLakeTaskGVR()),
	}
	cmd := &cobra.Command{
		Use:               "pause NAME",
		Short:             "Pause database synchronization.",
		Args:              cli.ExactArgs(1),
		Example:           pauseExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.Sync2FoxLakeTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args, func(task *v1alpha1.Sync2FoxLakeTask) {
				task.Spec.SyncDatabaseSpec.IsPaused = true
			}))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.Options.AddFlags(cmd)
	return cmd
}

func NewSync2FoxLakeResumeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &Sync2FoxLakePatchOptions{
		Factory: f,
		Options: patch.NewOptions(f, streams, types.Sync2FoxLakeTaskGVR()),
	}
	cmd := &cobra.Command{
		Use:               "resume NAME",
		Short:             "Resume database synchronization.",
		Args:              cli.ExactArgs(1),
		Example:           resumeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.Sync2FoxLakeTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args, func(task *v1alpha1.Sync2FoxLakeTask) {
				task.Spec.SyncDatabaseSpec.IsPaused = false
			}))
			util.CheckErr(o.Run(cmd))
		},
	}
	o.Options.AddFlags(cmd)
	return cmd
}
