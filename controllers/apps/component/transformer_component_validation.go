/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package component

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

var (
	defaultReplicasLimit = appsv1.ReplicasLimit{
		MinReplicas: 1,
		MaxReplicas: 16384,
	}
)

// componentValidationTransformer validates the consistency between spec & definition.
type componentValidationTransformer struct{}

var _ graph.Transformer = &componentValidationTransformer{}

func (t *componentValidationTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	comp := transCtx.Component

	var err error
	defer func() {
		setProvisioningStartedCondition(&comp.Status.Conditions, comp.Name, comp.Generation, err)
	}()

	if err = validateCompReplicas(comp, transCtx.CompDef); err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	if err = validateExternalManagedConfigSources(transCtx); err != nil {
		return intctrlutil.NewRequeueError(appsutil.RequeueDuration, err.Error())
	}
	// if err = validateSidecarContainers(comp, transCtx.CompDef); err != nil {
	// 	return newRequeueError(requeueDuration, err.Error())
	// }
	return nil
}

func validateCompReplicas(comp *appsv1.Component, compDef *appsv1.ComponentDefinition) error {
	replicasLimit := &defaultReplicasLimit
	// always respect the replicas limit if set.
	if compDef.Spec.ReplicasLimit != nil {
		replicasLimit = compDef.Spec.ReplicasLimit
	}

	replicas := comp.Spec.Replicas
	if replicas >= replicasLimit.MinReplicas && replicas <= replicasLimit.MaxReplicas {
		return nil
	}
	return replicasOutOfLimitError(replicas, *replicasLimit)
}

func replicasOutOfLimitError(replicas int32, replicasLimit appsv1.ReplicasLimit) error {
	return fmt.Errorf("replicas %d out-of-limit [%d, %d]", replicas, replicasLimit.MinReplicas, replicasLimit.MaxReplicas)
}

func validateExternalManagedConfigSources(transCtx *componentTransformContext) error {
	comp := transCtx.Component
	if len(comp.Spec.Configs) == 0 {
		return nil
	}

	externalManagedTemplates := map[string]component.SynthesizedFileTemplate{}
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		if tpl.Config && ptr.Deref(tpl.ExternalManaged, false) {
			externalManagedTemplates[tpl.Name] = tpl
		}
	}

	var externalManagedConfigs []appsv1.ClusterComponentConfig
	externalManagedConfigTemplates := map[string]component.SynthesizedFileTemplate{}
	for _, config := range comp.Spec.Configs {
		if config.Name == nil || config.ConfigMap == nil || config.ConfigMap.Name == "" {
			continue
		}
		tpl, externalManagedTemplate := externalManagedTemplates[*config.Name]
		if !externalManagedTemplate && !ptr.Deref(config.ExternalManaged, false) {
			continue
		}
		externalManagedConfigs = append(externalManagedConfigs, config)
		if externalManagedTemplate {
			externalManagedConfigTemplates[*config.Name] = tpl
		}
	}
	if len(externalManagedConfigs) == 0 {
		return nil
	}

	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return err
	}
	compName, err := component.ShortName(clusterName, comp.Name)
	if err != nil {
		return err
	}

	for _, config := range externalManagedConfigs {
		tpl := externalManagedConfigTemplates[*config.Name]
		if isExistingExternalManagedConfigSource(transCtx, config, tpl) {
			continue
		}
		if err := validateManagedConfigMapLabels(transCtx, config.ConfigMap.Name, clusterName, compName); err != nil {
			return err
		}
	}
	return nil
}

func isExistingExternalManagedConfigSource(transCtx *componentTransformContext, config appsv1.ClusterComponentConfig, tpl component.SynthesizedFileTemplate) bool {
	if transCtx.RunningWorkload == nil || config.ConfigMap == nil || tpl.VolumeName == "" {
		return false
	}
	runningITS, ok := transCtx.RunningWorkload.(*workloads.InstanceSet)
	if !ok || runningITS == nil {
		return false
	}
	for _, volume := range runningITS.Spec.Template.Spec.Volumes {
		if volume.Name == tpl.VolumeName && volume.ConfigMap != nil && volume.ConfigMap.Name == config.ConfigMap.Name {
			return true
		}
	}
	return false
}

func validateManagedConfigMapLabels(transCtx *componentTransformContext, cmName, clusterName, compName string) error {
	cm := &corev1.ConfigMap{}
	cmKey := types.NamespacedName{
		Namespace: transCtx.Component.Namespace,
		Name:      cmName,
	}
	if err := transCtx.Client.Get(transCtx.Context, cmKey, cm); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("configMap %q for external-managed config is not found", cmName)
		}
		return err
	}

	// Treat these common component labels as the apps-level handoff contract for
	// externally managed config sources. The component controller intentionally
	// does not inspect parameters-specific names, annotations, finalizers, or
	// owner references.
	expectedLabels := map[string]string{
		constant.AppManagedByLabelKey:   constant.AppName,
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: compName,
	}
	for key, expected := range expectedLabels {
		if actual := cm.Labels[key]; actual != expected {
			return fmt.Errorf("configMap %q for external-managed config must have label %q=%q, got %q",
				cmName, key, expected, actual)
		}
	}
	return nil
}
