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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/multicluster"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func collectInstanceConfigMaps(transCtx *componentTransformContext, cli client.Client, desired *workloads.InstanceSet, providerContexts []string) error {
	labels := constant.GetCompLabels(transCtx.SynthesizeComponent.ClusterName, transCtx.SynthesizeComponent.Name)
	authorities, err := instanceConfigMapAuthorities(transCtx, cli, labels)
	if err != nil {
		return configMapCollectionWait(err)
	}
	desiredNames := sets.New[string]()
	retainInstanceSetConfigMaps(desiredNames, desired)
	candidates := make([]*corev1.ConfigMap, 0, len(authorities))
	for _, cm := range authorities {
		if !desiredNames.Has(cm.Name) {
			candidates = append(candidates, cm)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	if its := transCtx.RunningWorkload; its != nil && its.Status.ObservedGeneration != its.Generation {
		return instanceConfigMapCleanupWait("waiting for workload to observe published instance configuration")
	}
	retained := desiredNames.Clone()
	retainInstanceSetConfigMaps(retained, transCtx.RunningWorkload)
	observed, err := observeInstanceConfigMapReferences(transCtx.Context, cli, transCtx.Component.Namespace, labels, "")
	if err != nil {
		return configMapCollectionWait(err)
	}
	retained.Insert(observed.references.UnsortedList()...)
	providerObservations := make(map[string]*instanceConfigMapObservations)
	for _, cm := range candidates {
		if retained.Has(cm.Name) {
			continue
		}
		mirrors, err := instanceConfigMapMirrors(transCtx, cli, cm, providerContexts)
		if err != nil {
			return configMapCollectionWait(err)
		}
		for _, mirror := range mirrors {
			observation, ok := providerObservations[mirror.placement]
			if !ok {
				observation, err = observeInstanceConfigMapReferences(transCtx.Context, cli, cm.Namespace, labels, mirror.placement)
				if err != nil {
					return configMapCollectionWait(err)
				}
				providerObservations[mirror.placement] = observation
			}
			retained.Insert(observation.references.UnsortedList()...)
		}
		if retained.Has(cm.Name) {
			continue
		}
		mirrorPresent := false
		for _, mirror := range mirrors {
			if mirror.configMap == nil || !isComponentInstanceConfigMapMirror(transCtx.Component, mirror.configMap) {
				continue
			}
			mirrorPresent = true
			if err := removeInstanceConfigMap(transCtx.Context, cli, mirror.configMap, mirror.placement); err != nil {
				return configMapCollectionWait(err)
			}
		}
		// Keep the authoritative placement record until every managed provider
		// copy has actually disappeared, including copies held by finalizers.
		if mirrorPresent {
			continue
		}
		if err := removeInstanceConfigMap(transCtx.Context, cli, cm, ""); err != nil {
			return configMapCollectionWait(err)
		}
	}
	return instanceConfigMapCleanupWait("waiting for retired instance configuration collection")
}

// The caller has already removed the InstanceSet. Keep its authority records
// until child runtimes and managed provider copies have disappeared as well.
func collectDeletingInstanceConfigMaps(transCtx *componentTransformContext, cli client.Client, labels map[string]string, providerContexts []string) error {
	authorities, err := instanceConfigMapAuthorities(transCtx, cli, labels)
	if err != nil {
		return err
	}
	if len(authorities) == 0 {
		return nil
	}
	control, err := observeInstanceConfigMapReferences(transCtx.Context, cli, transCtx.Component.Namespace, labels, "")
	if err != nil {
		return err
	}
	if control.workloadPresent {
		return intctrlutil.NewRequeueError(time.Second, "waiting for instance configuration consumers to disappear")
	}
	providerObservations := make(map[string]*instanceConfigMapObservations)
	copies := make([]instanceConfigMapCopy, 0)
	for _, authority := range authorities {
		mirrors, err := instanceConfigMapMirrors(transCtx, cli, authority, providerContexts)
		if err != nil {
			return err
		}
		for _, mirror := range mirrors {
			observation, ok := providerObservations[mirror.placement]
			if !ok {
				observation, err = observeInstanceConfigMapReferences(transCtx.Context, cli, authority.Namespace, labels, mirror.placement)
				if err != nil {
					return err
				}
				providerObservations[mirror.placement] = observation
			}
			if observation.workloadPresent {
				return intctrlutil.NewRequeueError(time.Second, "waiting for provider instances and Pods to disappear")
			}
			if mirror.configMap != nil && isComponentInstanceConfigMapMirror(transCtx.Component, mirror.configMap) {
				copies = append(copies, mirror)
			}
		}
	}
	for _, mirror := range copies {
		if err := removeInstanceConfigMap(transCtx.Context, cli, mirror.configMap, mirror.placement); err != nil {
			return err
		}
	}
	if len(copies) > 0 {
		return intctrlutil.NewRequeueError(time.Second, "waiting for instance configuration mirrors to disappear")
	}
	return nil
}

func instanceConfigMapAuthorities(transCtx *componentTransformContext, cli client.Client, labels map[string]string) ([]*corev1.ConfigMap, error) {
	selector := maps.Clone(labels)
	selector[instanceConfigMapLabelKey] = "true"
	list := &corev1.ConfigMapList{}
	if err := cli.List(transCtx.Context, list, client.InNamespace(transCtx.Component.Namespace), client.MatchingLabels(selector), multicluster.InControlContext()); err != nil {
		return nil, err
	}
	authorities := make([]*corev1.ConfigMap, 0, len(list.Items))
	for i := range list.Items {
		cm := &list.Items[i]
		if isCurrentComponentConfigMap(transCtx.Component, cm) {
			authorities = append(authorities, cm)
		}
	}
	return authorities, nil
}

type instanceConfigMapCopy struct {
	configMap *corev1.ConfigMap
	placement string
}

func instanceConfigMapMirrors(transCtx *componentTransformContext, cli client.Client, authority *corev1.ConfigMap, providerContexts []string) ([]instanceConfigMapCopy, error) {
	mirrors := make([]instanceConfigMapCopy, 0)
	placements := configMapPlacements(authority.Annotations[instanceConfigMapPlacementsAnnotationKey])
	// With no configured providers, routing falls back to the control client.
	configured := sets.New(providerContexts...)
	if missing := placements.Difference(configured); len(missing) > 0 {
		return nil, fmt.Errorf("instance configuration providers are not configured: %v", sets.List(missing))
	}
	for _, placement := range sets.List(placements) {
		current := &corev1.ConfigMap{}
		ctx := multicluster.IntoContext(transCtx.Context, placement)
		err := cli.Get(ctx, client.ObjectKeyFromObject(authority), current, multicluster.InDataContext())
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, err
		}
		if err == nil && (current.Name != authority.Name || current.Namespace != authority.Namespace || current.UID == "") {
			return nil, fmt.Errorf("incomplete instance configuration observation in provider %s", placement)
		}
		if apierrors.IsNotFound(err) {
			current = nil
		}
		mirrors = append(mirrors, instanceConfigMapCopy{configMap: current, placement: placement})
	}
	return mirrors, nil
}

type instanceConfigMapObservations struct {
	references      sets.Set[string]
	workloadPresent bool
}

func observeInstanceConfigMapReferences(ctx context.Context, cli client.Client, namespace string, labels map[string]string, placement string) (*instanceConfigMapObservations, error) {
	option := multicluster.InControlContext()
	if placement != "" {
		ctx = multicluster.IntoContext(ctx, placement)
		option = multicluster.InDataContext()
	}
	opts := []client.ListOption{client.InNamespace(namespace), client.MatchingLabels(labels), option}
	instances := &workloads.InstanceList{}
	if err := cli.List(ctx, instances, opts...); err != nil {
		return nil, err
	}
	pods := &corev1.PodList{}
	if err := cli.List(ctx, pods, opts...); err != nil {
		return nil, err
	}
	observation := &instanceConfigMapObservations{references: sets.New[string](), workloadPresent: len(instances.Items) > 0 || len(pods.Items) > 0}
	for _, instance := range instances.Items {
		retainPodConfigMaps(observation.references, &instance.Spec.Template.Spec)
	}
	for _, pod := range pods.Items {
		retainPodConfigMaps(observation.references, &pod.Spec)
	}
	return observation, nil
}

func removeInstanceConfigMap(ctx context.Context, cli client.Client, cm *corev1.ConfigMap, placement string) error {
	option := multicluster.InControlContext()
	if placement != "" {
		ctx = multicluster.IntoContext(ctx, placement)
		option = multicluster.InDataContext()
		cm = cm.DeepCopy()
		if cm.Annotations == nil {
			cm.Annotations = make(map[string]string)
		}
		cm.Annotations[constant.KBAppMultiClusterPlacementKey] = placement
	}
	if controllerutil.RemoveFinalizer(cm, constant.DBComponentFinalizerName) {
		// The multi-cluster writer does not return the new resource version.
		// Persist finalizer removal now and re-observe before a guarded delete.
		return client.IgnoreNotFound(cli.Update(ctx, cm, option))
	}
	return client.IgnoreNotFound(cli.Delete(ctx, cm, option, client.Preconditions{UID: &cm.UID, ResourceVersion: &cm.ResourceVersion}))
}

func instanceConfigMapCleanupWait(reason string) error {
	return intctrlutil.NewDelayedRequeueError(time.Second, reason)
}

func configMapCollectionWait(err error) error {
	return instanceConfigMapCleanupWait("waiting for complete instance configuration observations: " + err.Error())
}
