/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package custom

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	KBOpsActionNameLabelKey = "opsrequest.kubeblocks.io/action-name"

	KBEnvOpsName             = "KB_OPS_NAME"
	KbEnvOpsNamespace        = "KB_OPS_NAMESPACE"
	kbEnvCompHeadlessSVCName = "KB_COMP_HEADLESS_SVC_NAME"
	kbEnvCompSVCName         = "KB_COMP_SVC_NAME"
	kbEnvCompSVCPortPrefix   = "KB_COMP_SVC_PORT_"
	kbEnvAccountUserName     = "KB_ACCOUNT_USERNAME"
	kbEnvAccountPassword     = "KB_ACCOUNT_PASSWORD"
)

// buildComponentDefEnvs builds the env vars by the opsDefinition.spec.componentDefinitionRef
func buildComponentEnvs(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsDef *appsv1alpha1.OpsDefinition,
	env *[]corev1.EnvVar,
	comp *appsv1alpha1.ClusterComponentSpec) error {
	// inject built-in component env
	fullCompName := constant.GenerateClusterComponentName(cluster.Name, comp.Name)
	*env = append(*env, []corev1.EnvVar{
		{Name: constant.KBEnvClusterName, Value: cluster.Name},
		{Name: constant.KBEnvCompName, Value: comp.Name},
		{Name: constant.KBEnvClusterCompName, Value: fullCompName},
		{Name: constant.KBEnvCompReplicas, Value: strconv.Itoa(int(comp.Replicas))},
		{Name: kbEnvCompHeadlessSVCName, Value: constant.GenerateDefaultComponentHeadlessServiceName(cluster.Name, comp.Name)},
	}...)
	if len(opsDef.Spec.ComponentInfos) == 0 {
		return nil
	}
	// get component definition
	_, compDef, err := component.GetCompNCompDefByName(reqCtx.Ctx, cli, cluster.Namespace, fullCompName)
	if err != nil {
		return err
	}
	componentInfo := opsDef.GetComponentInfo(compDef.Name)
	if componentInfo == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf(`componentDefinition "%s" is not support for this operations`, compDef.Name))
	}

	*env = append(*env, corev1.EnvVar{Name: constant.KBEnvCompServiceVersion, Value: compDef.Spec.ServiceVersion})

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
	if componentInfo.AccountName != "" {
		accountSecretName := constant.GenerateAccountSecretName(cluster.Name, comp.Name, componentInfo.AccountName)
		*env = append(*env, corev1.EnvVar{Name: kbEnvAccountUserName, Value: componentInfo.AccountName})
		*env = append(*env, corev1.EnvVar{Name: kbEnvAccountPassword, ValueFrom: buildSecretKeyRef(accountSecretName, constant.AccountPasswdForSecret)})
	}

	// inject SVC and SVC ports
	if componentInfo.ServiceName != "" {
		for _, v := range compDef.Spec.Services {
			if v.Name != componentInfo.ServiceName {
				continue
			}
			*env = append(*env, corev1.EnvVar{Name: kbEnvCompSVCName, Value: constant.GenerateComponentServiceName(cluster.Name, comp.Name, v.ServiceName)})
			for _, port := range v.Spec.Ports {
				portName := strings.ReplaceAll(port.Name, "-", "_")
				*env = append(*env, corev1.EnvVar{Name: kbEnvCompSVCPortPrefix + strings.ToUpper(portName), Value: strconv.Itoa(int(port.Port))})
			}
			break
		}
	}
	return nil
}

// BuildEnvVars builds the env vars by the vars of the targetPodTemplate.
func buildEnvVars(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	targetPod *corev1.Pod,
	vars []appsv1alpha1.OpsEnvVar) ([]corev1.EnvVar, error) {
	if len(vars) == 0 {
		return []corev1.EnvVar{}, nil
	}
	// build vars
	var (
		envVars    []corev1.EnvVar
		err        error
		envFromMap = map[string]map[string]*corev1.EnvVar{}
	)
	buildEnvVarByEnvRef := func(envVarRef *appsv1alpha1.EnvVarRef) (*corev1.EnvVar, error) {
		container := intctrlutil.GetPodContainer(targetPod, envVarRef.TargetContainerName)
		if container == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find container "%s" of the pod "%s"`, envVarRef.TargetContainerName, targetPod.Name))
		}
		envVar := buildVarWithEnv(targetPod, container, envVarRef.EnvName)
		if envVar == nil {
			// if the var not found in container.env, try to find from container.envFrom.
			envMap := envFromMap[container.Name]
			if envMap != nil {
				return envMap[envVarRef.EnvName], nil
			}
			envMap, err = getEnvVarsFromEnvFrom(reqCtx.Ctx, cli, targetPod.Namespace, container)
			if err != nil {
				return nil, err
			}
			envFromMap[container.Name] = envMap
			envVar = envMap[envVarRef.EnvName]
		}
		return envVar, nil
	}

	for i := range vars {
		var envVar *corev1.EnvVar
		envVarRef := vars[i].ValueFrom.EnvVarRef
		if envVarRef != nil {
			envVar, err = buildEnvVarByEnvRef(envVarRef)
		} else if vars[i].ValueFrom.FieldRef != nil {
			envVar, err = buildVarWithFieldPath(targetPod, vars[i].ValueFrom.FieldRef.FieldPath)
		}
		if envVar == nil {
			return nil, intctrlutil.NewFatalError(fmt.Sprintf(`can not find the env "%s" in the container "%s"`, envVarRef.EnvName, envVarRef.TargetContainerName))
		}
		envVar.Name = vars[i].Name
		envVars = append(envVars, *envVar)
	}
	return envVars, nil
}

// BuildVarWithFieldPath builds the env var by field jsonpath of the pod.
func buildVarWithFieldPath(targetPod *corev1.Pod, fieldPath string) (*corev1.EnvVar, error) {
	path := jsonpath.New("jsonpath")
	if err := path.Parse(fmt.Sprintf("{%s}", fieldPath)); err != nil {
		return nil, fmt.Errorf("failed to parse fieldPath %s", fieldPath)
	}
	buff := bytes.NewBuffer([]byte{})
	if err := path.Execute(buff, targetPod); err != nil {
		return nil, fmt.Errorf("failed to execute fieldPath %s", fieldPath)
	}
	return &corev1.EnvVar{
		Value: buff.String(),
	}, nil
}

// GetEnvVarsFromEnvFrom gets the env var by the container envFrom.
func getEnvVarsFromEnvFrom(ctx context.Context, cli client.Client, podNamespace string, container *corev1.Container) (map[string]*corev1.EnvVar, error) {
	envMap := map[string]*corev1.EnvVar{}
	for _, env := range container.EnvFrom {
		prefix := env.Prefix
		if env.ConfigMapRef != nil {
			configMap := &corev1.ConfigMap{}
			if err := cli.Get(ctx, client.ObjectKey{Name: env.ConfigMapRef.Name, Namespace: podNamespace}, configMap); err != nil {
				return nil, err
			}
			for k, v := range configMap.Data {
				name := prefix + k
				envMap[name] = &corev1.EnvVar{Name: name, Value: v}
			}
		} else if env.SecretRef != nil {
			secret := &corev1.Secret{}
			if err := cli.Get(ctx, client.ObjectKey{Name: env.SecretRef.Name, Namespace: podNamespace}, secret); err != nil {
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
	return envMap, nil
}

// BuildVarWithEnv builds the env var by the container env.
func buildVarWithEnv(targetPod *corev1.Pod, container *corev1.Container, envName string) *corev1.EnvVar {
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

func buildActionPodEnv(reqCtx intctrlutil.RequestCtx,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	opsDef *appsv1alpha1.OpsDefinition,
	ops *appsv1alpha1.OpsRequest,
	comp *appsv1alpha1.ClusterComponentSpec,
	compCustomItem *appsv1alpha1.CustomOpsComponent,
	podInfoExtractor *appsv1alpha1.PodInfoExtractor,
	targetPod *corev1.Pod) ([]corev1.EnvVar, error) {
	var env = []corev1.EnvVar{
		{
			Name:  KBEnvOpsName,
			Value: ops.Name,
		},
		{
			Name:  KbEnvOpsNamespace,
			Value: ops.Namespace,
		},
	}
	// inject component and componentDef envs
	if err := buildComponentEnvs(reqCtx, cli, cluster, opsDef, &env, comp); err != nil {
		return nil, err
	}

	if podInfoExtractor != nil {
		// inject vars
		envVars, err := buildEnvVars(reqCtx, cli, targetPod, podInfoExtractor.Env)
		if err != nil {
			return nil, err
		}
		env = append(env, envVars...)
	}

	// inject params env
	params := compCustomItem.Parameters
	for i := range params {
		env = append(env, corev1.EnvVar{Name: params[i].Name, Value: params[i].Value})
	}
	return env, nil
}

func buildActionPodName(opsRequest *appsv1alpha1.OpsRequest,
	compName,
	actionName string,
	index,
	retries int) string {
	return fmt.Sprintf("%s-%s-%s-%s-%d-%d", opsRequest.UID[:8],
		common.CutString(opsRequest.Name, 30), compName, actionName, index, retries)
}

// getTargetPodInfoExtractor gets the target pod template.
func getTargetPodInfoExtractor(opsDef *appsv1alpha1.OpsDefinition, podInfoExtractorName string) *appsv1alpha1.PodInfoExtractor {
	for _, v := range opsDef.Spec.PodInfoExtractors {
		if podInfoExtractorName == v.Name {
			return &v
		}
	}
	return nil
}

func getTargetTemplateAndPod(ctx context.Context,
	cli client.Client,
	opsDef *appsv1alpha1.OpsDefinition,
	podInfoExtractorName,
	targetPodName,
	podNamespace string) (*appsv1alpha1.PodInfoExtractor, *corev1.Pod, error) {
	if targetPodName == "" {
		return nil, nil, nil
	}
	targetPodTemplate := getTargetPodInfoExtractor(opsDef, podInfoExtractorName)
	targetPod := &corev1.Pod{}
	if err := cli.Get(ctx,
		client.ObjectKey{Name: targetPodName, Namespace: podNamespace}, targetPod); err != nil {
		return nil, nil, err
	}
	return targetPodTemplate, targetPod, nil
}

// getTargetPods gets the target pods by podSelector of the targetPodTemplate.
func getTargetPods(
	ctx context.Context,
	cli client.Client,
	cluster *appsv1alpha1.Cluster,
	podSelector appsv1alpha1.PodSelector,
	compName string) ([]*corev1.Pod, error) {
	var (
		podList *corev1.PodList
		err     error
	)
	if podSelector.Role != "" {
		podList, err = component.GetComponentPodListWithRole(ctx, cli, *cluster, compName, podSelector.Role)
	} else {
		podList, err = component.GetComponentPodList(ctx, cli, *cluster, compName)
	}
	if err != nil {
		return nil, err
	}
	if len(podList.Items) == 0 {
		return nil, intctrlutil.NewFatalError("can not find any pod which matches the podSelector for the component " + compName)
	}
	sort.Sort(intctrlutil.ByPodName(podList.Items))
	var targetPods []*corev1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		targetPods = append(targetPods, pod)
		// Preferably select available pod.
		if podSelector.MultiPodSelectionPolicy == appsv1alpha1.Any && intctrlutil.IsAvailable(pod, 0) {
			return []*corev1.Pod{pod}, nil
		}
	}
	if podSelector.MultiPodSelectionPolicy == appsv1alpha1.Any {
		return targetPods[0:1], nil
	}
	return targetPods, nil
}

func buildLabels(clusterName, opsName, compName, actionName string) map[string]string {
	return map[string]string{
		constant.AppInstanceLabelKey:    clusterName,
		constant.OpsRequestNameLabelKey: opsName,
		constant.KBAppComponentLabelKey: compName,
		KBOpsActionNameLabelKey:         actionName,
		constant.AppManagedByLabelKey:   constant.AppName,
	}
}

func getNameFromObjectKey(objectKey string) string {
	strs := strings.Split(objectKey, "/")
	if len(strs) == 2 {
		return strs[1]
	}
	return objectKey
}
