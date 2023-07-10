package operations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/constant"
	componetutil "github.com/apecloud/kubeblocks/internal/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
	sqlchannelutil "github.com/apecloud/kubeblocks/internal/sqlchannel/util"
)

var _ OpsHandler = DataScriptOpsHandler{}
var _ error = &FastFaileError{}

// DataScriptOpsHandler handles DataScript operation, it is more like a one-time command operation.
type DataScriptOpsHandler struct {
}

// FastFaileError is a error type that will not retry the operation.
type FastFaileError struct {
	message string
}

func (e *FastFaileError) Error() string {
	return fmt.Sprintf("fail with message: %s", e.message)
}

func init() {
	// ToClusterPhase is not defined, because 'datascript' does not affect the cluster status.
	dataScriptOpsHander := DataScriptOpsHandler{}
	dataScriptBehavior := OpsBehaviour{
		FromClusterPhases:          []appsv1alpha1.ClusterPhase{appsv1alpha1.RunningClusterPhase},
		MaintainClusterPhaseBySelf: false,
		OpsHandler:                 dataScriptOpsHander,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.DataScriptType, dataScriptBehavior)
}

// Action implements OpsHandler.Action
// It will create a job to execute the script. It will fail fast if the script is not valid, or the target pod is not found.
func (o DataScriptOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	opsRequest := opsResource.OpsRequest
	cluster := opsResource.Cluster
	spec := opsRequest.Spec.ScriptSpec

	// get component
	component := cluster.Spec.GetComponentByName(spec.ComponentName)
	if component == nil {
		// we have checked component exists in validation, so this should not happen
		return &FastFaileError{message: fmt.Sprintf("component %s not found in cluster %s", spec.ComponentName, cluster.Name)}
	}

	clusterDef := &appsv1alpha1.ClusterDefinition{}
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Name: cluster.Spec.ClusterDefRef}, clusterDef); err != nil {
		if apierrors.IsNotFound(err) {
			// fail fast if cluster def does not exists
			return &FastFaileError{message: err.Error()}
		}
		return err
	}
	// get componentDef
	componentDef := clusterDef.GetComponentDefByName(component.ComponentDefRef)
	if componentDef == nil {
		return &FastFaileError{message: fmt.Sprintf("componentDef %s not found in clusterDef %s", component.ComponentDefRef, clusterDef.Name)}
	}

	// parse scripts
	script, err := getScriptContent(reqCtx, cli, spec)
	if err != nil {
		return &FastFaileError{message: err.Error()}
	}

	scriptRequest := sqlchannelutil.SQLChannelRequest{
		Operation: "exec",
		Metadata: map[string]interface{}{
			"sql": strings.Join(script, " "),
		},
	}
	// marsal json struct
	scriptRequestJSON, err := json.Marshal(scriptRequest)
	if err != nil {
		return &FastFaileError{message: err.Error()}
	}

	clusterObjectKey := client.ObjectKeyFromObject(cluster)
	podIP, err := getTargetPod(reqCtx, cli, clusterObjectKey, spec.ComponentName)
	if err != nil {
		return &FastFaileError{message: err.Error()}
	}

	// create job
	if job, err := createDataScriptJob(opsResource.Cluster, component, opsRequest,
		fmt.Sprintf(sqlchannelutil.DataScriptRequestTpl, podIP, componentDef.CharacterType, string(scriptRequestJSON))); err != nil {
		return err
	} else {
		err = cli.Create(reqCtx.Ctx, job)
		return err
	}
}

// ReconcileAction implements OpsHandler.ReconcileAction
// It will check the job status, and update the opsRequest status.
// If the job is neither completed nor failed, it will retry after 1 second.
// If the job is completed, it will return OpsSucceedPhase
// If the job is failed, it will return OpsFailedPhase.
func (o DataScriptOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	opsRequest := opsResource.OpsRequest
	cluster := opsResource.Cluster
	spec := opsRequest.Spec.ScriptSpec

	getStatusFromJobCondition := func(job *batchv1.Job) appsv1alpha1.OpsPhase {
		for _, condition := range job.Status.Conditions {
			if condition.Type == batchv1.JobComplete && condition.Status == corev1.ConditionTrue {
				return appsv1alpha1.OpsSucceedPhase
			} else if condition.Type == batchv1.JobFailed && condition.Status == corev1.ConditionTrue {
				return appsv1alpha1.OpsFailedPhase
			}
		}
		return appsv1alpha1.OpsRunningPhase
	}

	// retrieve job for this opsRequest
	jobList := &batchv1.JobList{}
	err := cli.List(reqCtx.Ctx, jobList, client.InNamespace(cluster.Namespace), client.MatchingLabels(getDataScriptJobLabels(cluster.Name, spec.ComponentName, opsRequest.Name)))
	if err != nil {
		return appsv1alpha1.OpsFailedPhase, 0, err
	}

	if len(jobList.Items) == 0 {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("job not found")
	}
	// check job status
	job := &jobList.Items[0]
	phase := getStatusFromJobCondition(job)
	// jobs are owned by opsRequest, so we don't need to delete them explicitly
	if phase == appsv1alpha1.OpsFailedPhase {
		return phase, 0, fmt.Errorf("job execution failed, please check the job log with `kubectl logs jobs/%s -n %s`", job.Name, job.Namespace)
	} else if phase == appsv1alpha1.OpsSucceedPhase {
		return phase, 0, nil
	}
	return phase, time.Second, nil
}

func (o DataScriptOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewDataScriptCondition(opsRes.OpsRequest), nil
}

func (o DataScriptOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
}

func (o DataScriptOpsHandler) GetRealAffectedComponentMap(opsRequest *appsv1alpha1.OpsRequest) realAffectedComponentMap {
	return realAffectedComponentMap(opsRequest.Spec.GetDataScriptComponentNameSet())
}

// getScriptContent will get script content from script or scriptFrom
func getScriptContent(reqCtx intctrlutil.RequestCtx, cli client.Client, spec *appsv1alpha1.ScriptSpec) ([]string, error) {
	script := make([]string, 0)
	if len(spec.Script) > 0 {
		script = append(script, spec.Script...)
	}
	if spec.ScriptFrom == nil {
		return script, nil
	}
	configMapsRefs := spec.ScriptFrom.ConfigMapRef
	secretRefs := spec.ScriptFrom.SecretRef

	if len(configMapsRefs) > 0 {
		obj := &corev1.ConfigMap{}
		for _, cm := range configMapsRefs {
			if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: cm.Name}, obj); err != nil {
				return nil, err
			}
			script = append(script, obj.Data[cm.Key])
		}
	}

	if len(secretRefs) > 0 {
		obj := &corev1.Secret{}
		for _, secret := range secretRefs {
			if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: secret.Name}, obj); err != nil {
				return nil, err
			}
			if obj.Data[secret.Key] == nil {
				return nil, fmt.Errorf("secret %s/%s does not have key %s", reqCtx.Req.Namespace, secret.Name, secret.Key)
			}
			secretData := string(obj.Data[secret.Key])
			// trim the last \n
			if len(secretData) > 0 && secretData[len(secretData)-1] == '\n' {
				secretData = secretData[:len(secretData)-1]
			}
			script = append(script, secretData)
		}
	}
	return script, nil
}

func getTargetPod(reqCtx intctrlutil.RequestCtx, cli client.Client, clusterObjectKey client.ObjectKey, componentName string) (string, error) {
	// get svc
	service := &corev1.Service{}
	serviceName := fmt.Sprintf("%s-%s", clusterObjectKey.Name, componentName)
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: clusterObjectKey.Namespace, Name: serviceName}, service); err != nil {
		return "", err
	}
	// get selector from service
	selector := service.Spec.Selector
	// get pods by selector
	podList := &corev1.PodList{}
	if err := cli.List(reqCtx.Ctx, podList, client.InNamespace(clusterObjectKey.Namespace), client.MatchingLabels(selector)); err != nil {
		return "", err
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pod found by service %s", componentName)
	}

	// get the first pod
	pod := podList.Items[0]
	podIP := pod.Status.PodIP
	if len(podIP) == 0 {
		return "", fmt.Errorf("pod %s has no podIP", pod.Name)
	}
	return podIP, nil
}

func createDataScriptJob(cluster *appsv1alpha1.Cluster, component *appsv1alpha1.ClusterComponentSpec, ops *appsv1alpha1.OpsRequest, scripts string) (*batchv1.Job, error) {
	jobName := fmt.Sprintf("%s-%s-%s", "script", ops.Name, component.Name)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cluster.Namespace,
		},
	}
	// set backoff limit to 0, so that the job will not be restarted
	job.Spec.BackoffLimit = pointer.Int32Ptr(0)
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.Template.Spec.Containers = []corev1.Container{
		{
			Name:    "datascript",
			Image:   viper.GetString(constant.KBToolsImage),
			Command: []string{"/bin/sh", "-c", scripts},
		},
	}

	// add labels
	job.Labels = getDataScriptJobLabels(cluster.Name, component.Name, ops.Name)
	// add tolerations
	tolerations, err := componetutil.BuildTolerations(cluster, component)
	if err != nil {
		return nil, &FastFaileError{message: err.Error()}
	}
	job.Spec.Template.Spec.Tolerations = tolerations
	// add owner reference
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err := controllerutil.SetOwnerReference(ops, job, scheme); err != nil {
		return nil, &FastFaileError{message: err.Error()}
	}
	return job, nil
}

func getDataScriptJobLabels(cluster, component, request string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    cluster,
		constant.KBAppComponentLabelKey: component,
		constant.OpsRequestNameLabelKey: request,
		constant.OpsRequestTypeLabelKey: string(appsv1alpha1.DataScriptType),
	}
}
