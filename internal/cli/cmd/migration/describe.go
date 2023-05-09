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

package migration

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	appv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	v1alpha1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

var (
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

	*v1alpha1.MigrationObjects
	genericclioptions.IOStreams
}

func newOptions(f cmdutil.Factory, streams genericclioptions.IOStreams) *describeOptions {
	return &describeOptions{
		factory:   f,
		IOStreams: streams,
		gvr:       types.MigrationTaskGVR(),
	}
}

func NewMigrationDescribeCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := newOptions(f, streams)
	cmd := &cobra.Command{
		Use:               "describe NAME",
		Short:             "Show details of a specific migration task.",
		Example:           DescribeExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.MigrationTaskGVR()),
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

	if _, err = IsMigrationCrdValidWithDynamic(&o.dynamic); err != nil {
		PrintCrdInvalidError(err)
	}

	if len(args) == 0 {
		return fmt.Errorf("migration task name should be specified")
	}
	o.names = args
	return nil
}

func (o *describeOptions) run() error {
	for _, name := range o.names {
		if err := o.describeMigration(name); err != nil {
			return err
		}
	}
	return nil
}

func (o *describeOptions) describeMigration(name string) error {
	var err error
	if o.MigrationObjects, err = getMigrationObjects(o, name); err != nil {
		return err
	}

	// MigrationTask Summary
	showTaskSummary(o.Task, o.Out)

	// MigrationTask Config
	showTaskConfig(o.Task, o.Out)

	// MigrationTemplate Summary
	showTemplateSummary(o.Template, o.Out)

	// Initialization Detail
	showInitialization(o.Task, o.Template, o.Jobs, o.Out)

	switch o.Task.Spec.TaskType {
	case v1alpha1.InitializationAndCdc, v1alpha1.CDC:
		// Cdc Detail
		showCdc(o.StatefulSets, o.Pods, o.Out)

		// Cdc Metrics
		showCdcMetrics(o.Task, o.Out)
	}

	fmt.Fprintln(o.Out)

	return nil
}

func getMigrationObjects(o *describeOptions, taskName string) (*v1alpha1.MigrationObjects, error) {
	obj := &v1alpha1.MigrationObjects{
		Task:     &v1alpha1.MigrationTask{},
		Template: &v1alpha1.MigrationTemplate{},
	}
	var err error
	taskGvr := types.MigrationTaskGVR()
	if err = APIResource(&o.dynamic, &taskGvr, taskName, o.namespace, obj.Task); err != nil {
		return nil, err
	}
	templateGvr := types.MigrationTemplateGVR()
	if err = APIResource(&o.dynamic, &templateGvr, obj.Task.Spec.Template, "", obj.Template); err != nil {
		return nil, err
	}
	listOpts := func() metav1.ListOptions {
		return metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", MigrationTaskLabel, taskName),
		}
	}
	if obj.Jobs, err = o.client.BatchV1().Jobs(o.namespace).List(context.Background(), listOpts()); err != nil {
		return nil, err
	}
	if obj.Pods, err = o.client.CoreV1().Pods(o.namespace).List(context.Background(), listOpts()); err != nil {
		return nil, err
	}
	if obj.StatefulSets, err = o.client.AppsV1().StatefulSets(o.namespace).List(context.Background(), listOpts()); err != nil {
		return nil, err
	}
	return obj, nil
}

func showTaskSummary(task *v1alpha1.MigrationTask, out io.Writer) {
	if task == nil {
		return
	}
	title := fmt.Sprintf("Name: %s\t Status: %s", task.Name, task.Status.TaskStatus)
	tbl := newTbl(out, title, "NAMESPACE", "CREATED-TIME", "START-TIME", "FINISHED-TIME")
	tbl.AddRow(task.Namespace, util.TimeFormatWithDuration(&task.CreationTimestamp, time.Second), util.TimeFormatWithDuration(task.Status.StartTime, time.Second), util.TimeFormatWithDuration(task.Status.FinishTime, time.Second))
	tbl.Print()
}

func showTaskConfig(task *v1alpha1.MigrationTask, out io.Writer) {
	if task == nil {
		return
	}
	tbl := newTbl(out, "\nMigration Config:")
	tbl.AddRow("source", fmt.Sprintf("%s:%s@%s/%s",
		task.Spec.SourceEndpoint.UserName,
		task.Spec.SourceEndpoint.Password,
		task.Spec.SourceEndpoint.Address,
		task.Spec.SourceEndpoint.DatabaseName,
	))
	tbl.AddRow("sink", fmt.Sprintf("%s:%s@%s/%s",
		task.Spec.SinkEndpoint.UserName,
		task.Spec.SinkEndpoint.Password,
		task.Spec.SinkEndpoint.Address,
		task.Spec.SinkEndpoint.DatabaseName,
	))
	tbl.AddRow("migration objects", task.Spec.MigrationObj.String(true))
	tbl.Print()
}

func showTemplateSummary(template *v1alpha1.MigrationTemplate, out io.Writer) {
	if template == nil {
		return
	}
	title := fmt.Sprintf("\nTemplate: %s\t", template.Name)
	tbl := newTbl(out, title, "DATABASE-MAPPING", "STATUS")
	tbl.AddRow(template.Spec.Description, template.Status.Phase)
	tbl.Print()
}

func showInitialization(task *v1alpha1.MigrationTask, template *v1alpha1.MigrationTemplate, jobList *batchv1.JobList, out io.Writer) {
	if len(jobList.Items) == 0 {
		return
	}
	sort.SliceStable(jobList.Items, func(i, j int) bool {
		jobName1 := jobList.Items[i].Name
		jobName2 := jobList.Items[j].Name
		order1, _ := strconv.ParseInt(string([]byte(jobName1)[strings.LastIndex(jobName1, "-")+1:]), 10, 8)
		order2, _ := strconv.ParseInt(string([]byte(jobName2)[strings.LastIndex(jobName2, "-")+1:]), 10, 8)
		return order1 < order2
	})
	cliStepOrder := BuildInitializationStepsOrder(task, template)
	tbl := newTbl(out, "\nInitialization:", "STEP", "NAMESPACE", "STATUS", "CREATED_TIME", "START-TIME", "FINISHED-TIME")
	if len(cliStepOrder) != len(jobList.Items) {
		return
	}
	for i, job := range jobList.Items {
		tbl.AddRow(cliStepOrder[i], job.Namespace, getJobStatus(job.Status.Conditions), util.TimeFormatWithDuration(&job.CreationTimestamp, time.Second), util.TimeFormatWithDuration(job.Status.StartTime, time.Second), util.TimeFormatWithDuration(job.Status.CompletionTime, time.Second))
	}
	tbl.Print()
}

func showCdc(statefulSets *appv1.StatefulSetList, pods *v1.PodList, out io.Writer) {
	if len(pods.Items) == 0 || len(statefulSets.Items) == 0 {
		return
	}
	tbl := newTbl(out, "\nCdc:", "NAMESPACE", "STATUS", "CREATED_TIME", "START-TIME")
	for _, pod := range pods.Items {
		if pod.Annotations[MigrationTaskStepAnnotation] != v1alpha1.StepCdc.String() {
			continue
		}
		tbl.AddRow(pod.Namespace, getCdcStatus(&statefulSets.Items[0], &pod), util.TimeFormatWithDuration(&pod.CreationTimestamp, time.Second), util.TimeFormatWithDuration(pod.Status.StartTime, time.Second))
	}
	tbl.Print()
}

func showCdcMetrics(task *v1alpha1.MigrationTask, out io.Writer) {
	if task.Status.Cdc.Metrics == nil || len(task.Status.Cdc.Metrics) == 0 {
		return
	}
	arr := make([]string, 0)
	for mKey := range task.Status.Cdc.Metrics {
		arr = append(arr, mKey)
	}
	sort.Strings(arr)
	tbl := newTbl(out, "\nCdc Metrics:")
	for _, k := range arr {
		tbl.AddRow(k, task.Status.Cdc.Metrics[k])
	}
	tbl.Print()
}

func getJobStatus(conditions []batchv1.JobCondition) string {
	if len(conditions) == 0 {
		return "-"
	} else {
		return string(conditions[len(conditions)-1].Type)
	}
}

func getCdcStatus(statefulSet *appv1.StatefulSet, cdcPod *v1.Pod) v1.PodPhase {
	if cdcPod.Status.Phase == v1.PodRunning &&
		statefulSet.Status.Replicas > statefulSet.Status.AvailableReplicas {
		if time.Now().Unix()-statefulSet.CreationTimestamp.Time.Unix() < 10*60 {
			return v1.PodPending
		} else {
			return v1.PodFailed
		}
	} else {
		return cdcPod.Status.Phase
	}
}
