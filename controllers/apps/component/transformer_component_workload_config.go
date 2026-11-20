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

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	instancecontroller "github.com/apecloud/kubeblocks/pkg/controller/instance"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	instanceConfigMapLabelKey                = "apps.kubeblocks.io/instance-config"
	instanceConfigMapPlacementsAnnotationKey = "apps.kubeblocks.io/instance-config-placements"
)

func dependOnInstanceConfigMaps(cli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) {
	referenced := boundConfigMapNames(its)
	for name := range referenced {
		cli.DependOn(dag, its, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: its.Namespace, Name: name}})
	}
}

func instanceConfigMapNames(its *workloads.InstanceSet) sets.Set[string] {
	base := sets.New[string]()
	retainPodConfigMaps(base, &its.Spec.Template.Spec)
	names := sets.New[string]()
	for _, instance := range its.Spec.Instances {
		for _, config := range instance.Configs {
			if config.Volume != nil && !base.Has(config.Volume.ConfigMapName) {
				names.Insert(config.Volume.ConfigMapName)
			}
		}
	}
	return names
}

func configMapPlacements(value string) sets.Set[string] {
	return sets.New(strings.FieldsFunc(value, func(r rune) bool { return r == ',' })...)
}

// Changed configuration must start with the runtime that produced it. Membership
// changes remain deferred to the normal scaling lifecycle after Start.
func publishStartingInstanceConfigs(target, desired *workloads.InstanceSet, legacyPolicy legacyConfigManagerPolicy) {
	templates := make(map[string]*workloads.InstanceTemplate, len(desired.Spec.Instances))
	for _, instance := range desired.Spec.Instances {
		if instance.GetReplicas() > 0 {
			templates[instance.Name] = instance.DeepCopy()
		}
	}
	changed := !sameStartingConfigs(target, desired, legacyPolicy)
	for _, instance := range target.Spec.Instances {
		var configs []workloads.ConfigTemplate
		if next := templates[instance.Name]; next != nil {
			configs = next.Configs
		}
		changed = changed || !reflect.DeepEqual(instance.Configs, configs)
	}
	if !changed {
		// Older workloads can adopt the binding for an existing mount without
		// applying unrelated deferred runtime changes.
		target.Spec.Configs = desired.Spec.Configs
		return
	}
	snapshot := desired.DeepCopy()
	target.Spec.Template = snapshot.Spec.Template
	target.Spec.Configs = snapshot.Spec.Configs
	target.Spec.VolumeClaimTemplates = snapshot.Spec.VolumeClaimTemplates
	target.Spec.LifecycleActions = snapshot.Spec.LifecycleActions
	target.Spec.InstanceAssistantObjects = snapshot.Spec.InstanceAssistantObjects
	for i, current := range target.Spec.Instances {
		next := templates[current.Name]
		if next == nil {
			// Removed or disabled templates temporarily use the desired base runtime.
			next = &workloads.InstanceTemplate{}
		}
		next.Name, next.Replicas, next.Ordinals = current.Name, current.Replicas, current.Ordinals
		target.Spec.Instances[i] = *next
	}
	if target.Annotations == nil {
		target.Annotations = make(map[string]string)
	}
	for _, key := range []string{constant.AppComponentLabelKey, constant.KBAppServiceVersionKey} {
		target.Annotations[key] = desired.Annotations[key]
	}
}

func sameStartingConfigs(current, desired *workloads.InstanceSet, legacyPolicy legacyConfigManagerPolicy) bool {
	if len(current.Spec.Configs) != len(desired.Spec.Configs) {
		return false
	}
	previous := make(map[string]workloads.ConfigTemplate, len(current.Spec.Configs))
	for _, config := range current.Spec.Configs {
		previous[config.Name] = config
	}
	volumes := sets.New[string]()
	for _, next := range desired.Spec.Configs {
		old, exists := previous[next.Name]
		if !exists {
			return false
		}
		if next.Volume != nil {
			volumes.Insert(next.Volume.VolumeName)
			if old.Volume == nil {
				// The Pod volume comparison below verifies this association.
				old.Volume = next.Volume
			}
		}
		if !reflect.DeepEqual(old, next) {
			return false
		}
	}
	if len(volumes) == 0 {
		return true
	}
	effective := desired
	if _, old := intctrlutil.GetContainerByName(current.Spec.Template.Spec.Containers, legacyConfigManagerContainerName); old != nil {
		effective = desired.DeepCopy()
		preserveLegacyConfigManagerPodSpec(current, desired, effective, legacyPolicy)
	}
	return reflect.DeepEqual(startingConfigMounts(&current.Spec.Template.Spec, volumes), startingConfigMounts(&effective.Spec.Template.Spec, volumes))
}

type configMounts struct {
	volumes    map[string]corev1.Volume
	containers map[string][]corev1.VolumeMount
	init       map[string][]corev1.VolumeMount
}

func startingConfigMounts(pod *corev1.PodSpec, names sets.Set[string]) configMounts {
	mounts := func(containers []corev1.Container) map[string][]corev1.VolumeMount {
		result := make(map[string][]corev1.VolumeMount)
		for _, container := range containers {
			for _, mount := range container.VolumeMounts {
				if names.Has(mount.Name) {
					result[container.Name] = append(result[container.Name], mount)
				}
			}
		}
		return result
	}
	result := configMounts{volumes: make(map[string]corev1.Volume), containers: mounts(pod.Containers), init: mounts(pod.InitContainers)}
	for _, volume := range pod.Volumes {
		if names.Has(volume.Name) {
			result.volumes[volume.Name] = volume
		}
	}
	return result
}

func boundConfigMapNames(its *workloads.InstanceSet) sets.Set[string] {
	names := sets.New[string]()
	volumes := sets.New[string]()
	for _, config := range its.Spec.Configs {
		if config.Volume != nil {
			volumes.Insert(config.Volume.VolumeName)
		}
	}
	for _, volume := range its.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil && volume.ConfigMap.Name != "" && volumes.Has(volume.Name) {
			names.Insert(volume.ConfigMap.Name)
		}
	}
	for _, instance := range its.Spec.Instances {
		for _, config := range instance.Configs {
			if config.Volume != nil {
				names.Insert(config.Volume.ConfigMapName)
			}
		}
	}
	return names
}

func isCurrentComponentConfigMap(comp *appsv1.Component, cm *corev1.ConfigMap) bool {
	if cm == nil || comp.UID == "" || cm.Labels[constant.KBAppConfigMapOwnerLabelKey] != string(comp.UID) {
		return false
	}
	owner := metav1.GetControllerOf(cm)
	return owner != nil && owner.UID == comp.UID && owner.Name == comp.Name &&
		owner.Kind == "Component" && owner.APIVersion == appsv1.GroupVersion.String()
}

func isComponentInstanceConfigMapMirror(comp *appsv1.Component, cm *corev1.ConfigMap) bool {
	return comp.UID != "" && len(cm.OwnerReferences) == 0 &&
		cm.Labels[constant.KBAppConfigMapOwnerLabelKey] == string(comp.UID)
}

func isLegacySharedConfigMapCopy(authority, candidate *corev1.ConfigMap) bool {
	return authority.Labels[instanceConfigMapLabelKey] != "true" &&
		instancecontroller.IsSharedAssistantConfigMap(candidate, authority.Labels[constant.AppInstanceLabelKey], authority.Labels[constant.KBAppComponentLabelKey])
}

func isManagedConfigMapCopy(comp *appsv1.Component, authority, candidate *corev1.ConfigMap) bool {
	return candidate != nil && (isComponentInstanceConfigMapMirror(comp, candidate) || isLegacySharedConfigMapCopy(authority, candidate))
}

func (t *componentWorkloadTransformer) prepareInstanceConfigMaps(transCtx *componentTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) error {
	if !ptr.Deref(its.Spec.EnableInstanceAPI, false) || ptr.Deref(its.Spec.Stop, false) || !multicluster.Enabled4Object(its) {
		return nil
	}
	placements := configMapPlacements(its.Annotations[constant.KBAppMultiClusterPlacementKey])
	managed := instanceConfigMapNames(its)
	for _, tpl := range transCtx.SynthesizeComponent.FileTemplates {
		if tpl.Config && !isExternalManaged(tpl) {
			managed.Insert(fileTemplateObjectName(transCtx.SynthesizeComponent, tpl.Name))
		}
	}
	for _, name := range sets.List(boundConfigMapNames(its)) {
		key := client.ObjectKey{Namespace: its.Namespace, Name: name}
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
		pending, _ := graphCli.FindMatchedVertex(dag, cm).(*model.ObjectVertex)
		if pending != nil && pending.Action != nil && (*pending.Action == model.CREATE || *pending.Action == model.UPDATE) {
			cm = pending.Obj.(*corev1.ConfigMap)
		} else {
			pending = nil
			if err := graphCli.Get(transCtx.Context, key, cm, multicluster.InControlContext()); err != nil {
				if apierrors.IsNotFound(err) && !managed.Has(name) {
					continue
				}
				return err
			}
		}
		if !isCurrentComponentConfigMap(transCtx.Component, cm) {
			if managed.Has(name) {
				return fmt.Errorf("instance configuration ConfigMap %s is not owned by Component %s", key, transCtx.Component.Name)
			}
			continue
		}
		if missing := placements.Difference(sets.New(t.configProviders...)); len(missing) > 0 {
			return fmt.Errorf("instance configuration providers are not configured: %v", sets.List(missing))
		}
		recorded := configMapPlacements(cm.Annotations[instanceConfigMapPlacementsAnnotationKey])
		if !recorded.IsSuperset(placements) {
			return intctrlutil.NewRequeueError(time.Second, "waiting for rendered instance configuration placement")
		}
		if pending != nil {
			var err error
			cm, err = t.publishConfigMapAuthority(transCtx.Context, dag, pending)
			if err != nil {
				return err
			}
		}
		for _, placement := range sets.List(placements) {
			if err := syncInstanceConfigMap(transCtx.Context, t.Client, transCtx.Component, cm, placement); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *componentWorkloadTransformer) publishConfigMapAuthority(ctx context.Context, dag *graph.DAG, pending *model.ObjectVertex) (*corev1.ConfigMap, error) {
	// Publish the current render before mirroring it. Rebuilding the plan first
	// can produce a new render on every reconcile.
	// Remove the planned write so it is not executed again if mirroring fails.
	dag.RemoveVertex(pending)
	cm := pending.Obj.(*corev1.ConfigMap).DeepCopy()
	if *pending.Action == model.CREATE {
		return cm, t.Client.Create(ctx, cm, multicluster.InControlContext())
	}
	old := pending.OriObj.(*corev1.ConfigMap)
	cm.UID, cm.ResourceVersion = old.UID, old.ResourceVersion
	return cm, t.Client.Update(ctx, cm, multicluster.InControlContext())
}

func syncInstanceConfigMap(ctx context.Context, cli client.Client, comp *appsv1.Component, config *corev1.ConfigMap, placement string) error {
	ctx = multicluster.IntoContext(ctx, placement)
	option := multicluster.InDataContext()
	key := client.ObjectKeyFromObject(config)
	current := &corev1.ConfigMap{}
	err := cli.Get(ctx, key, current, option)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		if current.Name != key.Name || current.Namespace != key.Namespace {
			return fmt.Errorf("cannot read instance configuration ConfigMap %s in placement %s", key, placement)
		}
		// A just-created authority may lack its assigned UID in the routed client's response.
		if (config.UID == "" || current.UID == config.UID) && isCurrentComponentConfigMap(comp, current) {
			return nil
		}
		if !isManagedConfigMapCopy(comp, config, current) {
			return fmt.Errorf("instance configuration ConfigMap %s in placement %s is not owned by Component %s", key, placement, comp.Name)
		}
		if !current.DeletionTimestamp.IsZero() {
			return intctrlutil.NewRequeueError(time.Second, "waiting for instance configuration mirror deletion")
		}
	}

	desired := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name, Labels: maps.Clone(config.Labels), Annotations: maps.Clone(config.Annotations)},
		Data:       config.Data, BinaryData: config.BinaryData,
	}
	if desired.Annotations == nil {
		desired.Annotations = make(map[string]string)
	}
	delete(desired.Annotations, instanceConfigMapPlacementsAnnotationKey)
	desired.Annotations[constant.KBAppMultiClusterPlacementKey] = placement
	if apierrors.IsNotFound(err) {
		return cli.Create(ctx, desired, option)
	}
	if reflect.DeepEqual(current.Data, desired.Data) && reflect.DeepEqual(current.BinaryData, desired.BinaryData) &&
		reflect.DeepEqual(current.Labels, desired.Labels) && reflect.DeepEqual(current.Annotations, desired.Annotations) {
		return nil
	}
	updated := current.DeepCopy()
	updated.Data, updated.BinaryData = desired.Data, desired.BinaryData
	updated.Labels, updated.Annotations = desired.Labels, desired.Annotations
	return cli.Update(ctx, updated, option)
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
