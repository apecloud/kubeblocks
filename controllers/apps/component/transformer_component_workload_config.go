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
	"fmt"
	"maps"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	instanceConfigMapLabelKey         = "apps.kubeblocks.io/instance-config"
	instanceConfigMapOwnerLabelKey    = "apps.kubeblocks.io/instance-config-owner"
	instanceConfigMapMirroredLabelKey = "apps.kubeblocks.io/instance-config-mirrored"
)

func (t *componentWorkloadTransformer) dependOnInstanceConfigMaps(transCtx *componentTransformContext,
	cli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) {
	for _, config := range its.Spec.Configs {
		if config.Volume != nil {
			cli.DependOn(dag, its, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: its.Namespace,
				Name:      config.Volume.ConfigMapName,
			}})
		}
	}
	referenced := sets.New[string]()
	retainInstanceSetConfigMaps(referenced, its)
	for _, cm := range transCtx.InstanceConfigMaps {
		if referenced.Has(cm.Name) {
			cli.DependOn(dag, its, cm)
		}
	}
}

// Start retains deferred workload changes, but must publish the files and hashes
// that the first replacement Pod will use before releasing the Stop barrier.
func publishStartingInstanceConfigs(target, desired *workloads.InstanceSet) {
	mounts := sets.New[string]()
	for _, volume := range target.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			mounts.Insert(volume.Name)
		}
	}
	for _, config := range desired.Spec.Configs {
		if config.Volume != nil && !mounts.Has(config.Volume.VolumeName) {
			return
		}
	}
	previousConfigMaps := sets.New[string]()
	retainInstanceSetConfigMaps(previousConfigMaps, target)
	target.Spec.Configs = desired.Spec.Configs
	configs := make(map[string][]workloads.ConfigTemplate, len(desired.Spec.Instances))
	for _, instance := range desired.Spec.Instances {
		configs[instance.Name] = instance.Configs
	}
	for i := range target.Spec.Instances {
		target.Spec.Instances[i].Configs = configs[target.Spec.Instances[i].Name]
	}
	for _, config := range desired.Spec.Configs {
		if config.Volume == nil {
			continue
		}
		for i := range target.Spec.Template.Spec.Volumes {
			volume := &target.Spec.Template.Spec.Volumes[i]
			if volume.Name == config.Volume.VolumeName && volume.ConfigMap != nil {
				volume.ConfigMap.Name = config.Volume.ConfigMapName
			}
		}
	}
	currentConfigMaps := sets.New[string]()
	retainInstanceSetConfigMaps(currentConfigMaps, target)
	assistants := make([]corev1.ObjectReference, 0, len(target.Spec.InstanceAssistantObjects))
	for _, ref := range target.Spec.InstanceAssistantObjects {
		if ref.Kind != "ConfigMap" || !previousConfigMaps.Has(ref.Name) {
			assistants = append(assistants, ref)
		}
	}
	for _, ref := range desired.Spec.InstanceAssistantObjects {
		if ref.Kind == "ConfigMap" && currentConfigMaps.Has(ref.Name) {
			assistants = append(assistants, ref)
		}
	}
	target.Spec.InstanceAssistantObjects = assistants
}

func isCurrentComponentConfigMap(comp *appsv1.Component, cm *corev1.ConfigMap) bool {
	if comp.UID == "" || cm.Labels[instanceConfigMapLabelKey] != "true" {
		return false
	}
	owner := metav1.GetControllerOf(cm)
	return owner != nil && owner.UID == comp.UID && owner.Name == comp.Name &&
		owner.Kind == "Component" && owner.APIVersion == appsv1.GroupVersion.String()
}

type instanceConfigMapDeletion struct {
	configMap *corev1.ConfigMap
	dataPlane bool
}

func isComponentInstanceConfigMapMirror(comp *appsv1.Component, cm *corev1.ConfigMap) bool {
	return comp.UID != "" && len(cm.OwnerReferences) == 0 &&
		cm.Labels[instanceConfigMapLabelKey] == "true" && cm.Labels[instanceConfigMapOwnerLabelKey] == string(comp.UID)
}

func syncInstanceConfigMaps(ctx context.Context, cli client.Client, comp *appsv1.Component,
	its *workloads.InstanceSet, configs []*corev1.ConfigMap) error {
	if its == nil || !ptr.Deref(its.Spec.EnableInstanceAPI, false) || !multicluster.Enabled4Object(its) {
		return nil
	}
	referenced := sets.New[string]()
	retainInstanceSetConfigMaps(referenced, its)
	for placement := range sets.New(strings.Split(its.Annotations[constant.KBAppMultiClusterPlacementKey], ",")...) {
		providerCtx := multicluster.IntoContext(ctx, placement)
		for _, config := range configs {
			if !referenced.Has(config.Name) {
				continue
			}
			if comp.UID == "" {
				return fmt.Errorf("cannot publish instance configuration without Component UID")
			}
			current := &corev1.ConfigMap{}
			err := cli.Get(providerCtx, client.ObjectKeyFromObject(config), current, multicluster.InDataContext())
			if err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			if err == nil && !isComponentInstanceConfigMapMirror(comp, current) {
				return fmt.Errorf("instance configuration ConfigMap %s/%s in placement %s is not owned by Component %s",
					config.Namespace, config.Name, placement, comp.Name)
			}
			if err == nil && !current.DeletionTimestamp.IsZero() {
				return intctrlutil.NewDelayedRequeueError(time.Second, "waiting for instance configuration mirror deletion")
			}
			desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Namespace: config.Namespace, Name: config.Name,
				Labels: maps.Clone(config.Labels), Annotations: maps.Clone(config.Annotations),
			}, Data: maps.Clone(config.Data), BinaryData: config.DeepCopy().BinaryData}
			if desired.Labels == nil {
				desired.Labels = map[string]string{}
			}
			desired.Labels[instanceConfigMapLabelKey] = "true"
			desired.Labels[instanceConfigMapOwnerLabelKey] = string(comp.UID)
			if desired.Annotations == nil {
				desired.Annotations = map[string]string{}
			}
			desired.Annotations[constant.KBAppMultiClusterPlacementKey] = placement
			if apierrors.IsNotFound(err) {
				if err = cli.Create(providerCtx, desired, multicluster.InDataContext()); err != nil {
					return err
				}
				continue
			}
			if reflect.DeepEqual(current.Data, desired.Data) && reflect.DeepEqual(current.BinaryData, desired.BinaryData) &&
				reflect.DeepEqual(current.Labels, desired.Labels) && reflect.DeepEqual(current.Annotations, desired.Annotations) {
				continue
			}
			updated := current.DeepCopy()
			updated.Data, updated.BinaryData = desired.Data, desired.BinaryData
			updated.Labels, updated.Annotations = desired.Labels, desired.Annotations
			if err = cli.Update(providerCtx, updated, multicluster.InDataContext()); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectDeletingInstanceConfigMaps(transCtx *componentTransformContext, labels map[string]string) error {
	selector := maps.Clone(labels)
	selector[instanceConfigMapLabelKey] = "true"
	authorities := &corev1.ConfigMapList{}
	if err := transCtx.Client.List(transCtx.Context, authorities, client.InNamespace(transCtx.Component.Namespace), client.MatchingLabels(selector)); err != nil {
		return configMapCollectionWait(err)
	}
	if !hasInstanceConfigMapMirrors(transCtx.Component, authorities.Items) {
		return nil
	}
	configs := &corev1.ConfigMapList{}
	if err := transCtx.Client.List(transCtx.Context, configs, client.InNamespace(transCtx.Component.Namespace),
		client.MatchingLabels(selector), multicluster.InDataContextUnspecifiedWithCompleteList()); err != nil {
		return configMapCollectionWait(err)
	}
	for i := range configs.Items {
		cm := &configs.Items[i]
		if isComponentInstanceConfigMapMirror(transCtx.Component, cm) {
			transCtx.InstanceConfigMapDeletions = append(transCtx.InstanceConfigMapDeletions,
				instanceConfigMapDeletion{configMap: cm, dataPlane: true})
		}
	}
	if len(transCtx.InstanceConfigMapDeletions) > 0 {
		return intctrlutil.NewDelayedRequeueError(time.Second, "waiting for instance configuration mirrors to be deleted")
	}
	return nil
}

func hasInstanceConfigMapMirrors(comp *appsv1.Component, authorities []corev1.ConfigMap) bool {
	if ptr.Deref(comp.Spec.EnableInstanceAPI, false) && multicluster.Enabled4Object(comp) {
		return true
	}
	for i := range authorities {
		cm := &authorities[i]
		if isCurrentComponentConfigMap(comp, cm) && cm.Labels[instanceConfigMapMirroredLabelKey] == "true" {
			return true
		}
	}
	return false
}

func (t *componentWorkloadTransformer) collectInstanceConfigMaps(transCtx *componentTransformContext, desired *workloads.InstanceSet) error {
	comp := transCtx.Component
	labels := constant.GetCompLabels(transCtx.SynthesizeComponent.ClusterName, transCtx.SynthesizeComponent.Name)
	labels[instanceConfigMapLabelKey] = "true"
	listOpts := []client.ListOption{client.InNamespace(comp.Namespace), client.MatchingLabels(labels)}
	controlMaps := &corev1.ConfigMapList{}
	if err := transCtx.Client.List(transCtx.Context, controlMaps, listOpts...); err != nil {
		return configMapCollectionWait(err)
	}
	desiredNames := sets.New[string]()
	for _, cm := range transCtx.InstanceConfigMaps {
		desiredNames.Insert(cm.Name)
	}
	retiring := false
	for i := range controlMaps.Items {
		cm := &controlMaps.Items[i]
		if isCurrentComponentConfigMap(comp, cm) && !desiredNames.Has(cm.Name) {
			retiring = true
			break
		}
	}
	if !retiring {
		return nil
	}

	// A placement may have been removed from the desired Component while an old
	// Instance or terminating Pod still uses it. Read every configured provider;
	// an unavailable provider must never look like an empty provider during GC.
	dataOpts := append(append([]client.ListOption{}, listOpts...), multicluster.InDataContextUnspecifiedWithCompleteList())
	dataMaps := &corev1.ConfigMapList{}
	hasMirrors := hasInstanceConfigMapMirrors(comp, controlMaps.Items)
	if hasMirrors {
		if err := transCtx.Client.List(transCtx.Context, dataMaps, dataOpts...); err != nil {
			return configMapCollectionWait(err)
		}
	}
	if its := transCtx.RunningWorkload; its != nil && its.Status.ObservedGeneration != its.Generation {
		return intctrlutil.NewDelayedRequeueError(time.Second, "waiting for workload to observe published instance configuration")
	}
	retained := sets.New[string]()
	for _, cm := range transCtx.InstanceConfigMaps {
		retained.Insert(cm.Name)
	}
	retainInstanceSetConfigMaps(retained, transCtx.RunningWorkload)
	retainInstanceSetConfigMaps(retained, desired)
	instances := &workloads.InstanceList{}
	if err := transCtx.Client.List(transCtx.Context, instances,
		client.InNamespace(comp.Namespace), client.MatchingLabels(constant.GetCompLabels(transCtx.SynthesizeComponent.ClusterName, transCtx.SynthesizeComponent.Name))); err != nil {
		return configMapCollectionWait(err)
	}
	for _, instance := range instances.Items {
		retainPodConfigMaps(retained, &instance.Spec.Template.Spec)
	}
	pods := &corev1.PodList{}
	podOpts := []client.ListOption{client.InNamespace(comp.Namespace),
		client.MatchingLabels(constant.GetCompLabels(transCtx.SynthesizeComponent.ClusterName, transCtx.SynthesizeComponent.Name))}
	if err := transCtx.Client.List(transCtx.Context, pods, podOpts...); err != nil {
		return configMapCollectionWait(err)
	}
	podLists := []*corev1.PodList{pods}
	if hasMirrors {
		dataPods := &corev1.PodList{}
		if err := transCtx.Client.List(transCtx.Context, dataPods,
			append(podOpts, multicluster.InDataContextUnspecifiedWithCompleteList())...); err != nil {
			return configMapCollectionWait(err)
		}
		podLists = append(podLists, dataPods)
	}
	for _, list := range podLists {
		for _, pod := range list.Items {
			retainPodConfigMaps(retained, &pod.Spec)
		}
	}
	pending := false
	mirrored := sets.New[string]()
	for i := range dataMaps.Items {
		cm := &dataMaps.Items[i]
		if isComponentInstanceConfigMapMirror(comp, cm) {
			mirrored.Insert(cm.Name)
		}
	}
	for i := range controlMaps.Items {
		cm := &controlMaps.Items[i]
		if !isCurrentComponentConfigMap(comp, cm) {
			continue
		}
		if retained.Has(cm.Name) {
			pending = pending || !desiredNames.Has(cm.Name)
			continue
		}
		if mirrored.Has(cm.Name) {
			pending = true
			continue
		}
		transCtx.InstanceConfigMapDeletions = append(transCtx.InstanceConfigMapDeletions, instanceConfigMapDeletion{configMap: cm})
	}
	for i := range dataMaps.Items {
		cm := &dataMaps.Items[i]
		if !isComponentInstanceConfigMapMirror(comp, cm) {
			continue
		}
		if retained.Has(cm.Name) {
			pending = pending || !desiredNames.Has(cm.Name)
			continue
		}
		transCtx.InstanceConfigMapDeletions = append(transCtx.InstanceConfigMapDeletions, instanceConfigMapDeletion{configMap: cm, dataPlane: true})
	}
	if pending {
		return intctrlutil.NewDelayedRequeueError(time.Second, "waiting for retired instance configuration references to disappear")
	}
	if len(transCtx.InstanceConfigMapDeletions) > 0 {
		return intctrlutil.NewDelayedRequeueError(time.Second, "confirming retired instance configuration collection")
	}
	return nil
}

func configMapCollectionWait(err error) error {
	return intctrlutil.NewDelayedRequeueError(time.Second, "waiting for complete instance configuration observations: "+err.Error())
}

func retainInstanceSetConfigMaps(retained sets.Set[string], its *workloads.InstanceSet) {
	if its == nil {
		return
	}
	retainPodConfigMaps(retained, &its.Spec.Template.Spec)
	for _, instance := range its.Spec.Instances {
		for _, config := range instance.Configs {
			if config.Volume != nil {
				retained.Insert(config.Volume.ConfigMapName)
			}
		}
	}
}

func retainPodConfigMaps(retained sets.Set[string], spec *corev1.PodSpec) {
	for _, volume := range spec.Volumes {
		if volume.ConfigMap != nil {
			retained.Insert(volume.ConfigMap.Name)
		}
		if volume.Projected != nil {
			for _, source := range volume.Projected.Sources {
				if source.ConfigMap != nil {
					retained.Insert(source.ConfigMap.Name)
				}
			}
		}
	}
}

func deleteInstanceConfigMaps(ctx context.Context, cli client.Client, deletions []instanceConfigMapDeletion) error {
	for _, deletion := range deletions {
		cm := deletion.configMap
		option := multicluster.InControlContext()
		deleteCtx := ctx
		if deletion.dataPlane {
			option = multicluster.InDataContext()
			deleteCtx = multicluster.IntoContext(ctx, cm.Annotations[constant.KBAppMultiClusterPlacementKey])
		}
		if controllerutil.RemoveFinalizer(cm, constant.DBComponentFinalizerName) {
			if err := cli.Update(deleteCtx, cm, option); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			// The multi-cluster writer does not return the updated resource version.
			// Re-observe before the guarded delete rather than using a stale version.
			continue
		}
		if err := cli.Delete(deleteCtx, cm, option, client.Preconditions{UID: &cm.UID, ResourceVersion: &cm.ResourceVersion}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
