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
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/dataprotection/utils"
)

const (
	kbEnvCompHeadlessSVCName = "KB_COMP_HEADLESS_SVC_NAME"
	kbEnvCompSVCName         = "KB_COMP_SVC_NAME"
	kbEnvCompSVCPortPrefix   = "KB_COMP_SVC_PORT_"
	kbEnvAccountUserName     = "KB_ACCOUNT_USERNAME"
	kbEnvAccountPassword     = "KB_ACCOUNT_PASSWORD"
)

type CustomOpsHandler struct{}

var _ OpsHandler = CustomOpsHandler{}

func init() {
	customBehaviour := OpsBehaviour{
		OpsHandler: CustomOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.CustomType, customBehaviour)
}

// ActionStartedCondition the started condition when handling the stop request.
func (c CustomOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	opsDefName := common.ToCamelCase(opsRes.OpsRequest.Spec.CustomSpec.OpsDefinitionRef)
	return &metav1.Condition{
		Type:               appsv1alpha1.ConditionTypeCustomOperation,
		Status:             metav1.ConditionTrue,
		Reason:             opsDefName + "Starting",
		LastTransitionTime: metav1.Now(),
		Message:            fmt.Sprintf("Start to handle %s on the Cluster: %s", opsDefName, opsRes.OpsRequest.Spec.ClusterRef),
	}, nil
}

func (c CustomOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	preConditions := opsRes.OpsDef.Spec.PreConditions
	customSpec := opsRes.OpsRequest.Spec.CustomSpec
	// 1. do preChecks
	for _, v := range preConditions {
		if v.Rule != nil {
			if err := c.checkExpression(reqCtx, cli, opsRes, v.Rule, customSpec.ComponentName); err != nil {
				return intctrlutil.NewFatalError(err.Error())
			}
		} else if v.Exec != nil {
			if err := c.checkExecAction(reqCtx, cli, opsRes, v.Exec); err != nil {
				return intctrlutil.NewFatalError(err.Error())
			}
		}
	}
	// 2. do job action
	params := customSpec.Params
	if len(params) == 0 {
		params = []map[string]string{nil}
	}
	for i := range params {
		job, err := c.buildJob(reqCtx, cli, opsRes, params[i], i)
		if err != nil {
			return err
		}
		if err = cli.Create(reqCtx.Ctx, job); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (c CustomOpsHandler) checkExpression(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	rule *appsv1alpha1.Rule,
	compName string) error {
	opsSpec := opsRes.OpsRequest.Spec
	componentObjName := constant.GenerateClusterComponentName(opsSpec.ClusterRef, compName)
	comp := &appsv1alpha1.Component{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: componentObjName, Namespace: opsRes.OpsRequest.Namespace}, comp); err != nil {
		return err
	}
	// get the built-in objects and covert the json tag
	getBuiltInObjs := func() (map[string]interface{}, error) {
		b, err := json.Marshal(map[string]interface{}{
			"cluster":   opsRes.Cluster,
			"component": comp,
			"params":    opsRes.OpsRequest.Spec.CustomSpec.Params,
		})
		if err != nil {
			return nil, err
		}
		data := map[string]interface{}{}
		if err = json.Unmarshal(b, &data); err != nil {
			return nil, err
		}
		return data, nil
	}

	data, err := getBuiltInObjs()
	if err != nil {
		return err
	}
	tmpl, err := template.New("opsDefTemplate").Parse(rule.Expression)
	if err != nil {
		return err
	}
	var buf strings.Builder
	if err = tmpl.Execute(&buf, data); err != nil {
		return err
	}
	if buf.String() == "false" {
		return fmt.Errorf(rule.Message)
	}
	return nil
}

func (c CustomOpsHandler) checkExecAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, exec *appsv1alpha1.PreConditionExec) error {
	// TODO: implement it
	// TODO: return needWaitingError to wait for job successfully.
	return nil
}

func (c CustomOpsHandler) getJobNamePrefix(opsName, compName string) string {
	jobName := fmt.Sprintf("%s-%s", opsName, compName)
	if len(jobName) > 61 {
		jobName = jobName[:61]
	}
	return jobName
}

func (c CustomOpsHandler) buildJob(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	opsRes *OpsResource,
	param map[string]string,
	index int) (*batchv1.Job, error) {
	var (
		customSpec  = opsRes.OpsRequest.Spec.CustomSpec
		compName    = customSpec.ComponentName
		clusterName = opsRes.Cluster.Name
		opsName     = opsRes.OpsRequest.Name
	)

	buildJobSpec := func() (*batchv1.JobSpec, error) {
		jobSpec := opsRes.OpsDef.Spec.JobSpec
		if jobSpec.BackoffLimit == nil {
			jobSpec.BackoffLimit = pointer.Int32(2)
		}
		comp := opsRes.Cluster.Spec.GetComponentByName(compName)
		if comp == nil {
			return nil, nil
		}
		// get component definition
		compDef, err := component.GetCompDefinition(reqCtx, cli, opsRes.Cluster, customSpec.ComponentName)
		if err != nil {
			return nil, err
		}

		// inject built-in env
		fullCompName := constant.GenerateClusterComponentName(clusterName, compName)
		env := []corev1.EnvVar{
			{Name: constant.KBEnvClusterName, Value: opsRes.Cluster.Name},
			{Name: constant.KBEnvCompName, Value: compName},
			{Name: constant.KBEnvClusterCompName, Value: fullCompName},
			{Name: constant.KBEnvCompReplicas, Value: strconv.Itoa(int(comp.Replicas))},
			{Name: constant.KBEnvCompServiceVersion, Value: compDef.Spec.ServiceVersion},
			{Name: kbEnvCompHeadlessSVCName, Value: constant.GenerateDefaultComponentHeadlessServiceName(clusterName, compName)},
		}
		varsRef := opsRes.OpsDef.Spec.VarsRef
		if compVarsRef, err := c.buildComponentDefEnvs(opsRes, &env, compDef, compName); err != nil {
			return nil, err
		} else if compVarsRef != nil {
			varsRef = compVarsRef
		}

		// inject vars
		envVars, err := c.buildEnvVars(reqCtx, cli, varsRef, opsRes, compName)
		if err != nil {
			return nil, err
		}
		env = append(env, envVars...)

		// inject params env
		for k, v := range param {
			env = append(env, corev1.EnvVar{Name: k, Value: v})
		}
		for i := range jobSpec.Template.Spec.Containers {
			jobSpec.Template.Spec.Containers[i].Env = append(jobSpec.Template.Spec.Containers[i].Env, env...)
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&jobSpec.Template.Spec.Containers[i])
		}
		if jobSpec.Template.Spec.RestartPolicy == "" {
			jobSpec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
		}
		if len(jobSpec.Template.Spec.Tolerations) == 0 {
			jobSpec.Template.Spec.Tolerations = comp.Tolerations
		}
		getSAName := func() string {
			if comp.ServiceAccountName != "" {
				return comp.ServiceAccountName
			}
			return constant.GenerateDefaultServiceAccountName(opsRes.Cluster.Name)
		}
		jobSpec.Template.Spec.ServiceAccountName = getSAName()
		return &jobSpec, nil
	}

	jobSpec, err := buildJobSpec()
	if err != nil {
		return nil, err
	}
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				constant.OpsRequestNameLabelKey: opsName,
				constant.KBAppComponentLabelKey: compName,
				constant.AppManagedByLabelKey:   constant.AppName,
			},
			Name:      fmt.Sprintf("%s-%d", c.getJobNamePrefix(opsName, compName), index),
			Namespace: opsRes.OpsRequest.Namespace,
		},
		Spec: *jobSpec,
	}
	// controllerutil.AddFinalizer(job, constant.OpsRequestFinalizerName)
	scheme, _ := appsv1alpha1.SchemeBuilder.Build()
	if err = utils.SetControllerReference(opsRes.OpsRequest, job, scheme); err != nil {
		return nil, err
	}
	return job, nil
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for stop opsRequest.
func (c CustomOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	compName := opsRes.OpsRequest.Spec.CustomSpec.ComponentName
	opsRequestPhase := opsRes.OpsRequest.Status.Phase
	jobList := &batchv1.JobList{}
	if err := cli.List(reqCtx.Ctx, jobList, client.InNamespace(opsRes.OpsRequest.Namespace),
		client.MatchingLabels{constant.OpsRequestNameLabelKey: opsRes.OpsRequest.Name}); err != nil {
		return opsRequestPhase, 0, err
	}
	compStatus, ok := opsRes.OpsRequest.Status.Components[compName]
	if !ok {
		compStatus = appsv1alpha1.OpsRequestComponentStatus{}
	}
	var (
		oldOpsRequest = opsRes.OpsRequest.DeepCopy()
		patch         = client.MergeFrom(oldOpsRequest)
		expectCount   = len(jobList.Items)
		failedCount   int
		completeCount int
	)
	for _, job := range jobList.Items {
		// handle the job progress
		progressDetail := appsv1alpha1.ProgressStatusDetail{ObjectKey: getProgressObjectKey(job.Kind, job.Name)}
		handleProgress := func(expectPhase appsv1alpha1.ProgressStatus, message string) {
			progressDetail.SetStatusAndMessage(expectPhase, message)
			setComponentStatusProgressDetail(opsRes.Recorder, opsRes.OpsRequest, &compStatus.ProgressDetails, progressDetail)
			opsRes.OpsRequest.Status.Components = map[string]appsv1alpha1.OpsRequestComponentStatus{
				compName: compStatus,
			}
			if isCompletedProgressStatus(expectPhase) {
				completeCount += 1
			}
			if expectPhase == appsv1alpha1.FailedProgressStatus {
				failedCount += 1
			}
		}
		// check if the job is completed
		handleProgress(appsv1alpha1.ProcessingProgressStatus, fmt.Sprintf("Processing %s", progressDetail.ObjectKey))
		for _, v := range job.Status.Conditions {
			if v.Status != corev1.ConditionTrue {
				continue
			}
			if v.Type == batchv1.JobComplete {
				handleProgress(appsv1alpha1.SucceedProgressStatus, fmt.Sprintf("Successfully handling %s", progressDetail.ObjectKey))
				break
			}
			if v.Type == batchv1.JobFailed {
				handleProgress(appsv1alpha1.FailedProgressStatus, fmt.Sprintf("%s: %s", v.Reason, v.Message))
				break
			}
		}
	}
	opsRes.OpsRequest.Status.Progress = fmt.Sprintf("%d/%d", completeCount, expectCount)
	if !reflect.DeepEqual(opsRes.OpsRequest.Status, oldOpsRequest.Status) {
		if err := cli.Status().Patch(reqCtx.Ctx, opsRes.OpsRequest, patch); err != nil {
			return opsRequestPhase, 0, err
		}
	}
	if expectCount == completeCount {
		if failedCount == 0 {
			opsRequestPhase = appsv1alpha1.OpsSucceedPhase
		} else {
			opsRequestPhase = appsv1alpha1.OpsFailedPhase
		}
	}
	return opsRequestPhase, 0, nil
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (c CustomOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (c CustomOpsHandler) buildComponentDefEnvs(opsRes *OpsResource, env *[]corev1.EnvVar, compDef *appsv1alpha1.ComponentDefinition, compName string) (*appsv1alpha1.VarsRef, error) {
	if len(opsRes.OpsDef.Spec.ComponentDefinitionRefs) == 0 {
		return nil, nil
	}
	compDefRef := opsRes.OpsDef.GetComponentDefRef(compDef.Name)
	if compDefRef == nil {
		return nil, intctrlutil.NewFatalError(fmt.Sprintf(`componentDefinition "%s" is not support for this operations`, compDef.Name))
	}

	buildSecretKeyRef := func(secretName, key string) *corev1.EnvVarSource {
		return &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: secretName,
				},
				Key: key,
			},
		}
	}
	// inject connect envs
	if compDefRef.AccountName != "" {
		accountSecretName := constant.GenerateAccountSecretName(opsRes.Cluster.Name, compName, compDefRef.AccountName)
		*env = append(*env, corev1.EnvVar{Name: kbEnvAccountUserName, Value: compDefRef.AccountName})
		*env = append(*env, corev1.EnvVar{Name: kbEnvAccountPassword, ValueFrom: buildSecretKeyRef(accountSecretName, constant.AccountPasswdForSecret)})
	}

	// inject SVC and SVC ports
	if compDefRef.ServiceName != "" {
		for _, v := range compDef.Spec.Services {
			if v.Name != compDefRef.ServiceName {
				continue
			}
			*env = append(*env, corev1.EnvVar{Name: kbEnvCompSVCName, Value: constant.GenerateComponentServiceName(opsRes.Cluster.Name, compName, v.ServiceName)})
			for _, port := range v.Spec.Ports {
				portName := strings.ReplaceAll(port.Name, "-", "_")
				*env = append(*env, corev1.EnvVar{Name: kbEnvCompSVCPortPrefix + strings.ToUpper(portName), Value: strconv.Itoa(int(port.Port))})
			}
			break
		}
	}
	return compDefRef.VarsRef, nil
}

func (c CustomOpsHandler) getTargetPod(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	varsRef *appsv1alpha1.VarsRef,
	opsRes *OpsResource,
	compName string) (*corev1.Pod, error) {
	podList, err := component.GetComponentPodList(reqCtx.Ctx, cli, *opsRes.Cluster, compName)
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, intctrlutil.NewFatalError("can not find any pod for the component " + compName)
	}
	var availablePod *corev1.Pod
	for _, v := range podList.Items {
		if intctrlutil.IsAvailable(&v, 0) {
			availablePod = &v
			break
		}
	}
	if availablePod == nil && varsRef.PodSelectionStrategy == appsv1alpha1.PreferredAvailable {
		if varsRef.PodSelectionStrategy == appsv1alpha1.Available {
			return nil, intctrlutil.NewFatalError("can not find any available pod for the component " + compName)
		}
		availablePod = &podList.Items[0]
	}
	return availablePod, nil
}

func (c CustomOpsHandler) buildEnvVars(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	varsRef *appsv1alpha1.VarsRef,
	opsRes *OpsResource,
	compName string) ([]corev1.EnvVar, error) {
	if varsRef == nil {
		return nil, nil
	}
	targetPod, err := c.getTargetPod(reqCtx, cli, varsRef, opsRes, compName)
	if err != nil {
		return nil, err
	}
	containerEnvMap := map[string]map[string]corev1.EnvVar{}
	for i := range targetPod.Spec.Containers {
		envMap := map[string]corev1.EnvVar{}
		container := targetPod.Spec.Containers[i]
		for j := range container.Env {
			envMap[container.Env[j].Name] = container.Env[j]
		}
		containerEnvMap[container.Name] = envMap
	}

	getContainer := func(containerName string) *corev1.Container {
		for i := range targetPod.Spec.Containers {
			container := targetPod.Spec.Containers[i]
			if container.Name == containerName {
				return &container
			}
		}
		return nil
	}

	getVarWithEnv := func(container *corev1.Container, envName string) *corev1.EnvVar {
		for i := range container.Env {
			env := container.Env[i]
			if env.Name != envName {
				continue
			}
			if env.ValueFrom != nil && env.ValueFrom.FieldRef != nil {
				// handle fieldRef
				value, _ := common.GetFieldRef(targetPod, env.ValueFrom)
				return &corev1.EnvVar{Name: envName, Value: value}
			}
			return &env
		}
		return nil
	}

	envFromMap := map[string]map[string]*corev1.EnvVar{}
	getVarWithEnvFrom := func(container *corev1.Container, envName string) (*corev1.EnvVar, error) {
		envMap := envFromMap[container.Name]
		if envMap != nil {
			return envMap[envName], nil
		}
		envMap = map[string]*corev1.EnvVar{}
		for _, env := range container.EnvFrom {
			prefix := env.Prefix
			if env.ConfigMapRef != nil {
				configMap := &corev1.ConfigMap{}
				if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: env.ConfigMapRef.Name, Namespace: targetPod.Namespace}, configMap); err != nil {
					return nil, err
				}
				for k, v := range configMap.Data {
					name := prefix + k
					envMap[name] = &corev1.EnvVar{Name: name, Value: v}
				}
			} else if env.SecretRef != nil {
				secret := &corev1.Secret{}
				if err = cli.Get(reqCtx.Ctx, client.ObjectKey{Name: env.SecretRef.Name, Namespace: targetPod.Namespace}, secret); err != nil {
					return nil, err
				}
				for k := range secret.Data {
					name := prefix + k
					envMap[name] = &corev1.EnvVar{Name: name, ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: env.SecretRef.Name},
						Key:                  k,
					}}}
				}
			}
		}
		envFromMap[container.Name] = envMap
		return envMap[envName], nil
	}

	// build vars
	existsEnvVarMap := map[string]struct{}{}
	var envVars []corev1.EnvVar
	for i := range varsRef.Vars {
		envVarRef := varsRef.Vars[i].ValueFrom.EnvVarRef
		container := getContainer(envVarRef.ContainerName)
		if container == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find container "%s" in the component "%s"`, envVarRef.ContainerName, compName))
		}
		envVar := getVarWithEnv(container, envVarRef.EnvName)
		if envVar == nil {
			// if the var not found in container.env, try to find from container.envFrom.
			envVar, err = getVarWithEnvFrom(container, envVarRef.EnvName)
			if err != nil {
				return nil, err
			}
		}
		if envVar == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find the env "%s" in the container "%s"`, envVarRef.EnvName, envVarRef.ContainerName))
		}
		envVar.Name = varsRef.Vars[i].Name
		envVars = append(envVars, *envVar)
		existsEnvVarMap[envVar.Name] = struct{}{}
	}
	return envVars, nil
}

// initOpsDefAndValidate inits the opsDefinition to OpsResource and validates if the opsRequest is valid.
func initOpsDefAndValidate(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	customSpec := opsRes.OpsRequest.Spec.CustomSpec
	if customSpec == nil {
		return intctrlutil.NewFatalError("spec.customSpec can not be empty if opsType is Custom.")
	}
	opsDef := &appsv1alpha1.OpsDefinition{}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: customSpec.OpsDefinitionRef}, opsDef); err != nil {
		return err
	}
	opsRes.OpsDef = opsDef
	// 1. validate OpenApV3Schema
	parametersSchema := opsDef.Spec.ParametersSchema
	for _, v := range customSpec.Params {
		// covert to type map[string]interface{}
		params, err := common.CoverStringToInterfaceBySchemaType(parametersSchema.OpenAPIV3Schema, v)
		if err != nil {
			return err
		}
		if parametersSchema != nil && parametersSchema.OpenAPIV3Schema != nil {
			if err = common.ValidateDataWithSchema(parametersSchema.OpenAPIV3Schema, params); err != nil {
				return err
			}
		}
	}
	// 2. validate component and componentDef
	comp := opsRes.Cluster.Spec.GetComponentByName(customSpec.ComponentName)
	if comp == nil {
		return intctrlutil.NewNotFound(`can not found component "%s" in cluster "%s"`, customSpec.ComponentName, opsRes.Cluster.Name)
	}
	compDef, err := component.GetCompDefinition(reqCtx, cli, opsRes.Cluster, customSpec.ComponentName)
	if err != nil {
		return err
	}
	if len(opsDef.Spec.ComponentDefinitionRefs) > 0 {
		var componentDefMatched bool
		for _, v := range opsDef.Spec.ComponentDefinitionRefs {
			if v.Name == compDef.Name {
				componentDefMatched = true
				break
			}
		}
		if !componentDefMatched {
			return intctrlutil.NewFatalError(fmt.Sprintf(`not supported componnet definition "%s"`, compDef.Name))
		}
	}
	return nil
}
