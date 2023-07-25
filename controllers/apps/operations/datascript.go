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
	"github.com/apecloud/kubeblocks/internal/sqlchannel/engine"
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

	clusterDef, err := getClusterDefByName(reqCtx.Ctx, cli, cluster.Spec.ClusterDefRef)
	if err != nil {
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

	// create jobs
	if job, err := buildDataScriptJob(reqCtx, cli, opsResource.Cluster, component, opsRequest, componentDef.CharacterType); err != nil {
		return err
	} else {
		return cli.Create(reqCtx.Ctx, job)
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

func getTargetService(reqCtx intctrlutil.RequestCtx, cli client.Client, clusterObjectKey client.ObjectKey, componentName string) (string, error) {
	// get svc
	service := &corev1.Service{}
	serviceName := fmt.Sprintf("%s-%s", clusterObjectKey.Name, componentName)
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: clusterObjectKey.Namespace, Name: serviceName}, service); err != nil {
		return "", err
	}
	return serviceName, nil
}

func buildDataScriptJob(reqCtx intctrlutil.RequestCtx, cli client.Client, cluster *appsv1alpha1.Cluster, component *appsv1alpha1.ClusterComponentSpec,
	ops *appsv1alpha1.OpsRequest, charType string) (*batchv1.Job, error) {
	engineForJob, err := engine.New(charType)
	if err != nil || engineForJob == nil {
		return nil, &FastFaileError{message: err.Error()}
	}

	envs := []corev1.EnvVar{}
	// parse kb host
	serviceName, err := getTargetService(reqCtx, cli, client.ObjectKeyFromObject(cluster), component.Name)
	if err != nil {
		return nil, &FastFaileError{message: err.Error()}
	}

	envs = append(envs, corev1.EnvVar{
		Name:  "KB_HOST",
		Value: serviceName,
	})

	// parse username and password
	secretFrom := ops.Spec.ScriptSpec.Secret
	if secretFrom == nil {
		secretFrom = &appsv1alpha1.ScriptSecret{
			Name:        fmt.Sprintf("%s-conn-credential", cluster.Name),
			PasswordKey: "password",
			UsernameKey: "username",
		}
	}
	// verify secrets exist
	if err := cli.Get(reqCtx.Ctx, types.NamespacedName{Namespace: reqCtx.Req.Namespace, Name: secretFrom.Name}, &corev1.Secret{}); err != nil {
		return nil, &FastFaileError{message: err.Error()}
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
		return nil, &FastFaileError{message: err.Error()}
	}

	envs = append(envs, corev1.EnvVar{
		Name:  "KB_SCRIPT",
		Value: strings.Join(scripts, "\n"),
	})

	jobCmdTpl, envVars, err := engineForJob.ExecuteCommand(scripts)
	if err != nil {
		return nil, &FastFaileError{message: err.Error()}
	}
	if envVars != nil {
		envs = append(envs, envVars...)
	}
	containerImg := viper.GetString(constant.KBDataScriptClientsImage)
	if len(ops.Spec.ScriptSpec.Image) != 0 {
		containerImg = ops.Spec.ScriptSpec.Image
	}
	if len(containerImg) == 0 {
		return nil, &FastFaileError{message: "image is empty"}
	}

	container := corev1.Container{
		Name:            "datascript",
		Image:           containerImg,
		ImagePullPolicy: corev1.PullPolicy(viper.GetString(constant.KBImagePullPolicy)),
		Command:         jobCmdTpl,
		Env:             envs,
	}

	jobName := fmt.Sprintf("%s-%s-%s", cluster.Name, "script", ops.Name)
	if len(jobName) > 63 {
		jobName = jobName[:63]
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: cluster.Namespace,
		},
	}

	// set backoff limit to 0, so that the job will not be restarted
	job.Spec.BackoffLimit = pointer.Int32Ptr(0)
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.Template.Spec.Containers = []corev1.Container{container}

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
