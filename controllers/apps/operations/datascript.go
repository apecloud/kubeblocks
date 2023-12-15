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

package operations

import (
	"fmt"
	"strings"
	"time"

	"github.com/sethvargo/go-password/password"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	componetutil "github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/lorry/engines/register"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

var _ OpsHandler = DataScriptOpsHandler{}

// DataScriptOpsHandler handles DataScript operation, it is more like a one-time command operation.
type DataScriptOpsHandler struct {
}

func init() {
	// ToClusterPhase is not defined, because 'datascript' does not affect the cluster status.
	dataScriptOpsHandler := DataScriptOpsHandler{}
	dataScriptBehavior := OpsBehaviour{
		FromClusterPhases: []appsv1alpha1.ClusterPhase{appsv1alpha1.RunningClusterPhase},
		OpsHandler:        dataScriptOpsHandler,
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
		return intctrlutil.NewFatalError(fmt.Sprintf("component %s not found in cluster %s", spec.ComponentName, cluster.Name))
	}

	clusterDef, err := getClusterDefByName(reqCtx.Ctx, cli, cluster.Spec.ClusterDefRef)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// fail fast if cluster def does not exists
			return intctrlutil.NewFatalError(err.Error())
		}
		return err
	}
	// get componentDef
	componentDef := clusterDef.GetComponentDefByName(component.ComponentDefRef)
	if componentDef == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf("componentDef %s not found in clusterDef %s", component.ComponentDefRef, clusterDef.Name))
	}

	// create jobs
	var jobs []*batchv1.Job
	if jobs, err = buildDataScriptJobs(reqCtx, cli, opsResource.Cluster, component, opsRequest, componentDef.CharacterType); err != nil {
		return err
	}
	for _, job := range jobs {
		if err = cli.Create(reqCtx.Ctx, job); err != nil {
			return err
		}
	}
	return nil
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

	meetsJobConditions := func(job *batchv1.Job, condType batchv1.JobConditionType, condStatus corev1.ConditionStatus) bool {
		for _, condition := range job.Status.Conditions {
			if condition.Type == condType && condition.Status == condStatus {
				return true
			}
		}
		return false
	}

	// retrieve job for this opsRequest
	jobList := &batchv1.JobList{}
	if err := cli.List(reqCtx.Ctx, jobList, client.InNamespace(cluster.Namespace), client.MatchingLabels(getDataScriptJobLabels(cluster.Name, spec.ComponentName, opsRequest.Name))); err != nil {
		return appsv1alpha1.OpsFailedPhase, 0, err
	} else if len(jobList.Items) == 0 {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("job not found")
	}

	var (
		expectedCount int
		succeedCount  int
		failedCount   int
	)

	expectedCount = len(jobList.Items)
	// check job status
	for _, job := range jobList.Items {
		if meetsJobConditions(&job, batchv1.JobComplete, corev1.ConditionTrue) {
			succeedCount++
		} else if meetsJobConditions(&job, batchv1.JobFailed, corev1.ConditionTrue) {
			failedCount++
		}
	}

	opsStatus := appsv1alpha1.OpsRunningPhase
	if succeedCount == expectedCount {
		opsStatus = appsv1alpha1.OpsSucceedPhase
	} else if failedCount+succeedCount == expectedCount {
		opsStatus = appsv1alpha1.OpsFailedPhase
	}

	patch := client.MergeFrom(opsRequest.DeepCopy())
	opsRequest.Status.Progress = fmt.Sprintf("%d/%d", succeedCount, expectedCount)

	// patch OpsRequest.status.components
	if err := cli.Status().Patch(reqCtx.Ctx, opsRequest, patch); err != nil {
		return opsStatus, time.Second, err
	}

	if succeedCount == expectedCount {
		return appsv1alpha1.OpsSucceedPhase, 0, nil
	} else if failedCount+succeedCount == expectedCount {
		return appsv1alpha1.OpsFailedPhase, 0, fmt.Errorf("%d job execution failed, please check the job log ", failedCount)
	}
	return appsv1alpha1.OpsRunningPhase, 5 * time.Second, nil
}

func (o DataScriptOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewDataScriptCondition(opsRes.OpsRequest), nil
}

func (o DataScriptOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsResource *OpsResource) error {
	return nil
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

func getTargetService(reqCtx intctrlutil.RequestCtx, cli client.Client, clusterObjectKey client.ObjectKey, componentName string) (string, error) {
	// get svc
	service := &corev1.Service{}
	serviceName := fmt.Sprintf("%s-%s", clusterObjectKey.Name, componentName)
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: clusterObjectKey.Namespace, Name: serviceName}, service); err != nil {
		return "", err
	}
	return serviceName, nil
}

func buildDataScriptJobs(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, component *appsv1alpha1.ClusterComponentSpec,
	ops *appsv1alpha1.OpsRequest, charType string) ([]*batchv1.Job, error) {
	engineForJob, err := register.NewClusterCommands(charType)
	if err != nil || engineForJob == nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}

	buildJob := func(endpoint string) (*batchv1.Job, error) {
		envs := []corev1.EnvVar{}

		envs = append(envs, corev1.EnvVar{
			Name:  "KB_HOST",
			Value: endpoint,
		})

		// parse username and password
		secretFrom := ops.Spec.ScriptSpec.Secret
		if secretFrom == nil {
			secretFrom = &appsv1alpha1.ScriptSecret{
				Name:        constant.GenerateDefaultConnCredential(cluster.Name),
				PasswordKey: "password",
				UsernameKey: "username",
			}
		}
		// verify secrets exist
		if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: secretFrom.Name}, &corev1.Secret{}); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}

		envs = append(envs, corev1.EnvVar{
			Name: "KB_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: secretFrom.UsernameKey,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretFrom.Name,
					},
				},
			},
		})
		envs = append(envs, corev1.EnvVar{
			Name: "KB_PASSWD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					Key: secretFrom.PasswordKey,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretFrom.Name,
					},
				},
			},
		})

		// parse scripts
		scripts, err := getScriptContent(reqCtx, cli, ops.Spec.ScriptSpec)
		if err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}

		envs = append(envs, corev1.EnvVar{
			Name:  "KB_SCRIPT",
			Value: strings.Join(scripts, "\n"),
		})

		jobCmdTpl, envVars, err := engineForJob.ExecuteCommand(scripts)
		if err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		if envVars != nil {
			envs = append(envs, envVars...)
		}
		containerImg := viper.GetString(constant.KBDataScriptClientsImage)
		if len(ops.Spec.ScriptSpec.Image) != 0 {
			containerImg = ops.Spec.ScriptSpec.Image
		}
		if len(containerImg) == 0 {
			return nil, intctrlutil.NewFatalError("image is empty")
		}

		container := corev1.Container{
			Name:            "datascript",
			Image:           containerImg,
			ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
			Command:         jobCmdTpl,
			Env:             envs,
		}
		randomStr, _ := password.Generate(4, 0, 0, true, false)
		jobName := fmt.Sprintf("%s-%s-%s-%s", cluster.Name, "script", ops.Name, randomStr)
		if len(jobName) > 63 {
			jobName = jobName[:63]
		}

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: cluster.Namespace,
			},
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(&container)
		// set backoff limit to 0, so that the job will not be restarted
		job.Spec.BackoffLimit = pointer.Int32(0)
		job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		job.Spec.Template.Spec.Containers = []corev1.Container{container}

		// add labels
		job.Labels = getDataScriptJobLabels(cluster.Name, component.Name, ops.Name)
		// add tolerations
		tolerations, err := componetutil.BuildTolerations(cluster, component)
		if err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		job.Spec.Template.Spec.Tolerations = tolerations
		// add owner reference
		scheme, _ := appsv1alpha1.SchemeBuilder.Build()
		if err := controllerutil.SetOwnerReference(ops, job, scheme); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		return job, nil
	}

	// parse kb host
	var endpoint string
	var job *batchv1.Job

	jobs := make([]*batchv1.Job, 0)
	if ops.Spec.ScriptSpec.Selector == nil {
		if endpoint, err = getTargetService(reqCtx, cli, client.ObjectKeyFromObject(cluster), component.Name); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		if job, err = buildJob(endpoint); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		}
		jobs = append(jobs, job)
		return jobs, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(ops.Spec.ScriptSpec.Selector)
	if err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	}

	pods := &corev1.PodList{}
	if err = cli.List(reqCtx.Ctx, pods, client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			constant.AppInstanceLabelKey:    cluster.Name,
			constant.KBAppComponentLabelKey: component.Name,
		},
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return nil, intctrlutil.NewFatalError(err.Error())
	} else if len(pods.Items) == 0 {
		return nil, intctrlutil.NewFatalError(err.Error())
	}

	for _, pod := range pods.Items {
		endpoint = pod.Status.PodIP
		if job, err = buildJob(endpoint); err != nil {
			return nil, intctrlutil.NewFatalError(err.Error())
		} else {
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func getDataScriptJobLabels(cluster, component, request string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    cluster,
		constant.KBAppComponentLabelKey: component,
		constant.OpsRequestNameLabelKey: request,
		constant.OpsRequestTypeLabelKey: string(appsv1alpha1.DataScriptType),
	}
}
