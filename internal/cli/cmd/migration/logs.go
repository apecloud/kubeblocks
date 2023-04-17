package migration

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cmdlogs "k8s.io/kubectl/pkg/cmd/logs"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/polymorphichelpers"

	"github.com/apecloud/kubeblocks/internal/cli/exec"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	migrationv1 "github.com/apecloud/kubeblocks/internal/cli/types/migrationapi"
	"github.com/apecloud/kubeblocks/internal/cli/util"
)

type LogsOptions struct {
	taskName string
	step     string
	Client   *kubernetes.Clientset
	Dynamic  dynamic.Interface
	*exec.ExecOptions
	logOptions cmdlogs.LogsOptions
}

func NewMigrationLogsCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	l := &LogsOptions{
		ExecOptions: exec.NewExecOptions(f, streams),
		logOptions: cmdlogs.LogsOptions{
			Tail:      -1,
			IOStreams: streams,
		},
	}
	cmd := &cobra.Command{
		Use:               "logs NAME",
		Short:             "Access migration task log file.",
		Example:           LogsExample,
		ValidArgsFunction: util.ResourceNameCompletionFunc(f, types.MigrationTaskGVR()),
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(l.ExecOptions.Complete())
			util.CheckErr(l.complete(f, cmd, args))
			util.CheckErr(l.validate())
			util.CheckErr(l.runLogs())
		},
	}
	l.addFlags(cmd)
	return cmd
}

func (o *LogsOptions) addFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&o.step, "step", "", "Specify the step. Allow values: precheck,init-struct,init-data,cdc")

	o.logOptions.AddFlags(cmd)
}

// complete customs complete function for logs
func (o *LogsOptions) complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("migration task name should be specified")
	}
	if len(args) > 0 {
		o.taskName = args[0]
	}
	if o.step == "" {
		return fmt.Errorf("migration task step should be specified")
	}
	var err error
	o.logOptions.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Dynamic, err = f.DynamicClient()
	if err != nil {
		return err
	}

	o.Client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	_, err = IsMigrationCrdValidWithDynamic(&o.Dynamic)
	if errors.IsNotFound(err) {
		return fmt.Errorf("datamigration crd is not install")
	} else if err != nil {
		return err
	}

	taskObj, err := o.getMigrationObjects(o.taskName)
	if err != nil {
		return fmt.Errorf("failed to find the migrationtask")
	}
	pod := o.getPodByStep(taskObj, strings.TrimSpace(o.step))
	if pod == nil {
		return fmt.Errorf("migrationtask[%s] step[%s] 's pod not found", taskObj.Task.Name, o.step)
	}
	o.logOptions.RESTClientGetter = f
	o.logOptions.LogsForObject = polymorphichelpers.LogsForObjectFn
	o.logOptions.Object = pod
	o.logOptions.Options, _ = o.logOptions.ToLogOptions()
	o.Pod = pod

	return nil
}

func (o *LogsOptions) validate() error {
	if len(o.taskName) == 0 {
		return fmt.Errorf("migration task name must be specified")
	}

	if o.logOptions.LimitBytes < 0 {
		return fmt.Errorf("--limit-bytes must be greater than 0")
	}
	if o.logOptions.Tail < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}
	if len(o.logOptions.SinceTime) > 0 && o.logOptions.SinceSeconds != 0 {
		return fmt.Errorf("at most one of `sinceTime` or `sinceSeconds` may be specified")
	}
	logsOptions, ok := o.logOptions.Options.(*corev1.PodLogOptions)
	if !ok {
		return fmt.Errorf("unexpected logs options object")
	}
	if logsOptions.SinceSeconds != nil && *logsOptions.SinceSeconds < int64(0) {
		return fmt.Errorf("--since must be greater than 0")
	}
	if logsOptions.TailLines != nil && *logsOptions.TailLines < -1 {
		return fmt.Errorf("--tail must be greater than or equal to -1")
	}
	return nil
}

func (o *LogsOptions) getMigrationObjects(taskName string) (*migrationv1.MigrationObjects, error) {
	obj := &migrationv1.MigrationObjects{
		Task:     &migrationv1.MigrationTask{},
		Template: &migrationv1.MigrationTemplate{},
	}
	var err error
	taskGvr := types.MigrationTaskGVR()
	if err = APIResource(&o.Dynamic, &taskGvr, taskName, o.logOptions.Namespace, obj.Task); err != nil {
		return nil, err
	}
	templateGvr := types.MigrationTemplateGVR()
	if err = APIResource(&o.Dynamic, &templateGvr, obj.Task.Spec.Template, "", obj.Template); err != nil {
		return nil, err
	}
	listOpts := func() metav1.ListOptions {
		return metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", MigrationTaskLabel, taskName),
		}
	}
	if obj.Pods, err = o.Client.CoreV1().Pods(o.logOptions.Namespace).List(context.Background(), listOpts()); err != nil {
		return nil, err
	}
	return obj, nil
}

func (o *LogsOptions) runLogs() error {
	requests, err := o.logOptions.LogsForObject(o.logOptions.RESTClientGetter, o.logOptions.Object, o.logOptions.Options, 60*time.Second, false)
	if err != nil {
		return err
	}
	for _, request := range requests {
		if err := cmdlogs.DefaultConsumeRequest(request, o.Out); err != nil {
			if !o.logOptions.IgnoreLogErrors {
				return err
			}
			fmt.Fprintf(o.Out, "error: %v\n", err)
		}
	}
	return nil
}

func (o *LogsOptions) getPodByStep(taskObj *migrationv1.MigrationObjects, step string) *corev1.Pod {
	if taskObj == nil || len(taskObj.Pods.Items) == 0 {
		return nil
	}
	switch step {
	case migrationv1.CliStepCdc.String():
		for _, pod := range taskObj.Pods.Items {
			if pod.Annotations[MigrationTaskStepAnnotation] == migrationv1.StepCdc.String() {
				return &pod
			}
		}
	case migrationv1.CliStepPreCheck.String(), migrationv1.CliStepInitStruct.String(), migrationv1.CliStepInitData.String():
		stepArr := BuildInitializationStepsOrder(taskObj.Task, taskObj.Template)
		orderNo := "-1"
		for index, stepByTemplate := range stepArr {
			if step == stepByTemplate {
				orderNo = strconv.Itoa(index)
				break
			}
		}
		for _, pod := range taskObj.Pods.Items {
			if pod.Annotations[SerialJobOrderAnnotation] != "" &&
				pod.Annotations[SerialJobOrderAnnotation] == orderNo {
				return &pod
			}
		}
	}
	return nil
}
