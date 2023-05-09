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
	"fmt"
	"io"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/sync2foxlakeapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
	describeExample = templates.Examples(`
		# describe a sync2foxlake task named mytask
		kbcli sync2foxlake describe mytask
	`)

	newTbl = func(out io.Writer, title string, header ...interface{}) *printer.TablePrinter {
		fmt.Fprintln(out, title)
		tbl := printer.NewTablePrinter(out)
		tbl.SetHeader(header...)
		return tbl
	}
)

type describeOptions struct {
	factory   cmdutil.Factory
	client    clientset.Interface
	dynamic   dynamic.Interface
	namespace string

	// resource type and names
	gvr   schema.GroupVersionResource
	names []string

	*v1alpha1.Sync2FoxLakeTask
	genericclioptions.IOStreams
}

func newOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *describeOptions {
	return &describeOptions{
		factory:   f,
		IOStreams: streams,
		gvr:       types.Sync2FoxLakeTaskGVR(),
	}
}

func NewSync2FoxLakeDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "describe NAME",
		Short:             "Show details of a specific sync2foxlake task.",
		Example:           describeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.Sync2FoxLakeTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(args))
			util.CheckErr(o.run())
		},
	}
	return cmd
}

func (o *describeOptions) complete(args []string) error {
	var err error

	if o.client, err = o.factory.KubernetesClientSet(); err != nil {
		return err
	}

	if o.dynamic, err = o.factory.DynamicClient(); err != nil {
		return err
	}

	if o.namespace, _, err = o.factory.ToRawKubeConfigLoader().Namespace(); err != nil {
		return err
	}

	if len(args) == 0 {
		return fmt.Errorf("missing sync2foxlake task name")
	}
	o.names = args
	return nil
}

func (o *describeOptions) run() error {
	for _, name := range o.names {
		if err := o.describe(name); err != nil {
			return err
		}
	}
	return nil
}

func (o *describeOptions) describe(name string) error {
	var err error
	o.Sync2FoxLakeTask = &v1alpha1.Sync2FoxLakeTask{}
	obj, err := o.dynamic.Resource(types.Sync2FoxLakeTaskGVR()).Namespace(o.namespace).Get(context.Background(), name, metav1.GetOptions{}, "")
	if err != nil {
		return err
	}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, o.Sync2FoxLakeTask)
	if err != nil {
		return err
	}

	// Sync2FoxLakeTask Summary
	o.showTaskSummary(o.Sync2FoxLakeTask, o.Out)

	// Sync2FoxLakeTask Config
	o.showTaskConfig(o.Sync2FoxLakeTask, o.Out)

	// Sync2FoxLakeTask Status
	o.showTaskStatus(o.Sync2FoxLakeTask, o.Out)
	return nil
}

func (o *describeOptions) showTaskSummary(task *v1alpha1.Sync2FoxLakeTask, out io.Writer) {
	if task == nil {
		return
	}
	title := fmt.Sprintf("Name: %s\t Status: %s", task.Name, task.Status.TaskStatus)
	tbl := newTbl(out, title, "NAMESPACE", "CREATED-TIME")
	tbl.AddRow(task.Namespace, task.CreationTimestamp)
	tbl.Print()
}

func (o *describeOptions) showTaskConfig(task *v1alpha1.Sync2FoxLakeTask, out io.Writer) {
	if task == nil {
		return
	}
	tbl := newTbl(out, "\nSync2FoxLake Config:")
	if task.Spec.SourceEndpoint.EndpointType == AddressEndpointType {
		tbl.AddRow("source", fmt.Sprintf("%s:%s@%s",
			task.Spec.SourceEndpoint.UserName,
			task.Spec.SourceEndpoint.Password,
			task.Spec.SourceEndpoint.Endpoint,
		))
	} else {
		tbl.AddRow("sink", task.Spec.SourceEndpoint.Endpoint)
	}
	if task.Spec.SinkEndpoint.EndpointType == AddressEndpointType {
		tbl.AddRow("sink", fmt.Sprintf("%s:%s@%s",
			task.Spec.SinkEndpoint.UserName,
			task.Spec.SinkEndpoint.Password,
			task.Spec.SinkEndpoint.Endpoint,
		))
	} else {
		tbl.AddRow("sink", task.Spec.SinkEndpoint.Endpoint)
	}
	tbl.AddRow("database", task.Status.Database)
	tbl.AddRow("engine", task.Spec.SyncDatabaseSpec.Engine)
	tbl.AddRow("datasource_type", task.Spec.SyncDatabaseSpec.DatabaseType)
	tbl.AddRow("datasource_endpoint", task.Spec.SourceEndpoint.Endpoint)
	tbl.AddRow("datasource_user", task.Spec.SourceEndpoint.UserName)
	tbl.AddRow("datasource_password", task.Spec.SinkEndpoint.Password)
	tbl.AddRow("database_selected", task.Spec.SyncDatabaseSpec.DatabaseSelected)
	tbl.AddRow("lag", task.Spec.SyncDatabaseSpec.Lag)
	tbl.AddRow("quota", task.Spec.SyncDatabaseSpec.Quota)

	tbl.Print()
}

func (o *describeOptions) showTaskStatus(task *v1alpha1.Sync2FoxLakeTask, out io.Writer) {
	if task == nil {
		return
	}
	tbl := newTbl(out, "\nSyncDatabase Status:")
	tbl.AddRow("DATABASE", task.Status.Database)
	tbl.AddRow("APPLIED_SEQUENCE_ID", task.Status.AppliedSequenceID)
	tbl.AddRow("TARGET_SEQUENCE_ID", task.Status.TargetSequenceID)
	tbl.AddRow("STATUS", task.Status.TaskStatus)

	tbl.Print()
}
