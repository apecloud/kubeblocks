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
	"context"
	"maps"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func collectInstanceConfigMaps(transCtx *componentTransformContext, cli client.Client, desired *workloads.InstanceSet) error {
	labels := constant.GetCompLabels(transCtx.SynthesizeComponent.ClusterName, transCtx.SynthesizeComponent.Name)
	selector := maps.Clone(labels)
	selector[instanceConfigMapLabelKey] = "true"
	namespace := client.InNamespace(transCtx.Component.Namespace)
	configMaps := &corev1.ConfigMapList{}
	if err := cli.List(transCtx.Context, configMaps, namespace, client.MatchingLabels(selector)); err != nil {
		return configMapCollectionWait(err)
	}

	retained := sets.New[string]()
	retainInstanceSetConfigMaps(retained, desired)
	candidates := make([]*corev1.ConfigMap, 0, len(configMaps.Items))
	for i := range configMaps.Items {
		cm := &configMaps.Items[i]
		if !retained.Has(cm.Name) && metav1.IsControlledBy(cm, transCtx.Component) {
			candidates = append(candidates, cm)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	running := transCtx.RunningWorkload
	if running != nil && running.Status.ObservedGeneration != running.Generation {
		return instanceConfigMapCleanupWait("waiting for workload to observe published instance configuration")
	}
	retainInstanceSetConfigMaps(retained, running)
	instances := &workloads.InstanceList{}
	if err := cli.List(transCtx.Context, instances, namespace, client.MatchingLabels(labels)); err != nil {
		return configMapCollectionWait(err)
	}
	for _, instance := range instances.Items {
		retainPodConfigMaps(retained, &instance.Spec.Template.Spec)
		for _, assistant := range instance.Spec.InstanceAssistantObjects {
			if cm := assistant.ConfigMap; cm != nil && cm.Namespace == transCtx.Component.Namespace {
				retained.Insert(cm.Name)
			}
		}
	}
	pods := &corev1.PodList{}
	if err := cli.List(transCtx.Context, pods, namespace, client.MatchingLabels(labels)); err != nil {
		return configMapCollectionWait(err)
	}
	for _, pod := range pods.Items {
		retainPodConfigMaps(retained, &pod.Spec)
	}

	for _, cm := range candidates {
		if !retained.Has(cm.Name) {
			if err := removeInstanceConfigMap(transCtx.Context, cli, cm); err != nil {
				return configMapCollectionWait(err)
			}
		}
	}
	return instanceConfigMapCleanupWait("waiting for retired instance configuration collection")
}

func removeInstanceConfigMap(ctx context.Context, cli client.Client, cm *corev1.ConfigMap) error {
	if controllerutil.RemoveFinalizer(cm, constant.DBComponentFinalizerName) {
		return client.IgnoreNotFound(cli.Update(ctx, cm))
	}
	return client.IgnoreNotFound(cli.Delete(ctx, cm, client.Preconditions{UID: &cm.UID, ResourceVersion: &cm.ResourceVersion}))
}

func instanceConfigMapCleanupWait(reason string) error {
	return intctrlutil.NewDelayedRequeueError(time.Second, reason)
}

func configMapCollectionWait(err error) error {
	return instanceConfigMapCleanupWait("waiting for complete instance configuration observations: " + err.Error())
}
