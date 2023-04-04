package migration

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
	"github.com/spf13/cobra"
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
		Example:           MigrationDescribeExample,
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

	_, err = IsMigrationCrdValidWithDynamic(&o.dynamic)
	if errors.IsNotFound(err) {
		return fmt.Errorf("datamigration crd is not install")
	} else if err != nil {
		return err
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

	// MigrationTemplate Summary
	showTemplateSummary(o.Template, o.Out)

	// Initialization Detail
	showInitialization(o.Task, o.Template, o.Jobs, o.Out)

	switch o.Task.Spec.TaskType {
	case v1alpha1.InitializationAndCdc, v1alpha1.CDC:
		// Cdc Detail
		showCdc(o.Pods, o.Out)

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
	if err = ApiResource(&o.dynamic, &taskGvr, taskName, o.namespace, obj.Task); err != nil {
		return nil, err
	}
	templateGvr := types.MigrationTemplateGVR()
	if err = ApiResource(&o.dynamic, &templateGvr, obj.Task.Spec.Template, "", obj.Template); err != nil {
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
	return obj, nil
}

func showTaskSummary(task *v1alpha1.MigrationTask, out io.Writer) {
	if task == nil {
		return
	}
	title := fmt.Sprintf("Name: %s\t Status: %s", task.Name, task.Status.TaskStatus)
	tbl := newTbl(out, title, "NAMESPACE", "CREATED-TIME", "START-TIME", "FINISHED-TIME")
	tbl.AddRow(task.Namespace, TimeFormat(&task.CreationTimestamp), TimeFormat(task.Status.StartTime), TimeFormat(task.Status.FinishTime))
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
	if len(jobList.Items) <= 0 {
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
		tbl.AddRow(cliStepOrder[i], job.Namespace, getJobStatus(job.Status.Conditions), TimeFormat(&job.CreationTimestamp), TimeFormat(job.Status.StartTime), TimeFormat(job.Status.CompletionTime))
	}
	tbl.Print()
}

func showCdc(pods *v1.PodList, out io.Writer) {
	if len(pods.Items) <= 0 {
		return
	}
	tbl := newTbl(out, "\nCdc:", "NAMESPACE", "STATUS", "CREATED_TIME", "START-TIME")
	for _, pod := range pods.Items {
		if pod.Annotations[MigrationTaskStepAnnotation] != v1alpha1.StepCdc.String() {
			continue
		}
		tbl.AddRow(pod.Namespace, pod.Status.Phase, TimeFormat(&pod.CreationTimestamp), TimeFormat(pod.Status.StartTime))
	}
	tbl.Print()
}

func showCdcMetrics(task *v1alpha1.MigrationTask, out io.Writer) {
	if task.Status.Cdc.Metrics == nil || len(task.Status.Cdc.Metrics) <= 0 {
		return
	}
	arr := make([]string, 0)
	for mKey := range task.Status.Cdc.Metrics {
		arr = append(arr, mKey)
	}
	tbl := newTbl(out, "\nCdc Metrics:")
	for _, k := range arr {
		tbl.AddRow(k, task.Status.Cdc.Metrics[k])
	}
	tbl.Print()
}

func getJobStatus(conditions []batchv1.JobCondition) string {
	if len(conditions) <= 0 {
		return "-"
	} else {
		return string(conditions[len(conditions)-1].Type)
	}
}
