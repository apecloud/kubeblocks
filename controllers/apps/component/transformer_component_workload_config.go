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
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const (
	instanceConfigMapLabelKey                = "apps.kubeblocks.io/instance-config"
	instanceConfigMapOwnerLabelKey           = "apps.kubeblocks.io/instance-config-owner"
	instanceConfigMapPlacementsAnnotationKey = "apps.kubeblocks.io/instance-config-placements"
)

func dependOnInstanceConfigMaps(cli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) {
	referenced := instanceConfigMapNames(its)
	for _, config := range its.Spec.Configs {
		if config.Volume != nil {
			referenced.Insert(config.Volume.ConfigMapName)
		}
	}
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

func isComponentInstanceConfigMapMirror(comp *appsv1.Component, cm *corev1.ConfigMap) bool {
	return comp.UID != "" && len(cm.OwnerReferences) == 0 &&
		cm.Labels[instanceConfigMapLabelKey] == "true" && cm.Labels[instanceConfigMapOwnerLabelKey] == string(comp.UID)
}

func (t *componentWorkloadTransformer) prepareInstanceConfigMaps(transCtx *componentTransformContext,
	graphCli model.GraphClient, dag *graph.DAG, its *workloads.InstanceSet) error {
	if !ptr.Deref(its.Spec.EnableInstanceAPI, false) || ptr.Deref(its.Spec.Stop, false) || !multicluster.Enabled4Object(its) {
		return nil
	}
	placements := sets.List(configMapPlacements(its.Annotations[constant.KBAppMultiClusterPlacementKey]))
	for _, name := range sets.List(instanceConfigMapNames(its)) {
		key := client.ObjectKey{Namespace: its.Namespace, Name: name}
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
		if graphCli.IsAction(dag, cm, model.ActionCreatePtr()) || graphCli.IsAction(dag, cm, model.ActionUpdatePtr()) {
			return intctrlutil.NewRequeueError(time.Second, "waiting for rendered instance configuration")
		}
		cm = &corev1.ConfigMap{}
		if err := graphCli.Get(transCtx.Context, key, cm); err != nil {
			return err
		}
		if !isCurrentComponentConfigMap(transCtx.Component, cm) {
			return fmt.Errorf("instance configuration ConfigMap %s is not owned by Component %s", key, transCtx.Component.Name)
		}
		recorded := configMapPlacements(cm.Annotations[instanceConfigMapPlacementsAnnotationKey])
		for _, placement := range placements {
			if !recorded.Has(placement) {
				return intctrlutil.NewRequeueError(time.Second, "waiting for rendered instance configuration placement")
			}
			if err := syncInstanceConfigMap(transCtx.Context, t.Client, transCtx.Component, cm, placement); err != nil {
				return err
			}
		}
	}
	return nil
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
		if current.UID == config.UID && isCurrentComponentConfigMap(comp, current) {
			return nil
		}
		if !isComponentInstanceConfigMapMirror(comp, current) {
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
