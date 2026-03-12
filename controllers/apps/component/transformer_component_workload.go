/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"reflect"
	"strings"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// componentWorkloadTransformer handles component workload generation
type componentWorkloadTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentWorkloadTransformer{}

func (t *componentWorkloadTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if isCompDeleting(transCtx.ComponentOrig) {
		return nil
	}

	compDef := transCtx.CompDef
	comp := transCtx.Component
	synthesizeComp := transCtx.SynthesizeComponent

	runningITS := transCtx.RunningWorkload
	protoITS, err := component.BuildInstanceSet(synthesizeComp, compDef)
	if err != nil {
		return err
	}
	transCtx.ProtoWorkload = protoITS

	if err = t.reconcileWorkload(transCtx.Context, t.Client, synthesizeComp, comp, runningITS, protoITS); err != nil {
		return err
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)
	if runningITS == nil {
		if protoITS != nil {
			if err := setCompOwnershipNFinalizer(comp, protoITS); err != nil {
				return err
			}
			graphCli.Create(dag, protoITS)
			return nil
		}
	} else {
		if protoITS == nil {
			graphCli.Delete(dag, runningITS)
		} else {
			err = t.handleUpdate(transCtx, graphCli, dag, synthesizeComp, comp, runningITS, protoITS)
		}
	}
	return err
}

func (t *componentWorkloadTransformer) reconcileWorkload(ctx context.Context, cli client.Reader,
	synthesizedComp *component.SynthesizedComponent, comp *appsv1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	// if runningITS already exists, the image changes in protoITS will be
	// rollback to the original image in `checkNRollbackProtoImages`.
	// So changing registry configs won't affect existing clusters.
	for i, container := range protoITS.Spec.Template.Spec.Containers {
		protoITS.Spec.Template.Spec.Containers[i].Image = intctrlutil.ReplaceImageRegistry(container.Image)
	}
	for i, container := range protoITS.Spec.Template.Spec.InitContainers {
		protoITS.Spec.Template.Spec.InitContainers[i].Image = intctrlutil.ReplaceImageRegistry(container.Image)
	}

	t.buildInstanceSetPlacementAnnotation(comp, protoITS)

	if err := t.reconcileReplicasStatus(ctx, cli, synthesizedComp, runningITS, protoITS); err != nil {
		return err
	}

	return nil
}

func (t *componentWorkloadTransformer) buildInstanceSetPlacementAnnotation(comp *appsv1.Component, its *workloads.InstanceSet) {
	if comp.Annotations != nil {
		placement := comp.Annotations[constant.KBAppMultiClusterPlacementKey]
		if len(placement) > 0 {
			if its.Annotations == nil {
				its.Annotations = make(map[string]string)
			}
			its.Annotations[constant.KBAppMultiClusterPlacementKey] = placement
		}
	}
}

func (t *componentWorkloadTransformer) reconcileReplicasStatus(ctx context.Context, cli client.Reader,
	synthesizedComp *component.SynthesizedComponent, runningITS, protoITS *workloads.InstanceSet) error {
	var (
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)

	// HACK: sync replicas status from runningITS to protoITS
	component.BuildReplicasStatus(runningITS, protoITS)

	replicas, err := func() ([]string, error) {
		pods, err := component.ListOwnedPods(ctx, cli, namespace, clusterName, compName)
		if err != nil {
			return nil, err
		}
		podNameSet := sets.New[string]()
		for _, pod := range pods {
			podNameSet.Insert(pod.Name)
		}

		desiredPodNames, err := component.GetDesiredPodNamesByITS(runningITS, protoITS)
		if err != nil {
			return nil, err
		}
		desiredPodNameSet := sets.New(desiredPodNames...)

		return desiredPodNameSet.Intersection(podNameSet).UnsortedList(), nil
	}()
	if err != nil {
		return err
	}

	hasMemberJoinDefined, hasDataActionDefined := hasMemberJoinNDataActionDefined(synthesizedComp.LifecycleActions.ComponentLifecycleActions)
	return component.StatusReplicasStatus(protoITS, replicas, hasMemberJoinDefined, hasDataActionDefined)
}

func (t *componentWorkloadTransformer) handleUpdate(transCtx *componentTransformContext, cli model.GraphClient, dag *graph.DAG,
	synthesizedComp *component.SynthesizedComponent, comp *appsv1.Component, runningITS, protoITS *workloads.InstanceSet) error {
	start, stop := t.handleWorkloadStartNStop(transCtx, synthesizedComp, runningITS, &protoITS)
	if !(start || stop) {
		// postpone the update of the workload until the component is back to running.
		if err := t.handleWorkloadUpdate(transCtx, dag, synthesizedComp, comp, runningITS, protoITS); err != nil {
			return err
		}
	}

	objCopy := copyAndMergeITS(runningITS, protoITS, legacyConfigManagerRequired(comp))
	if objCopy != nil {
		cli.Update(dag, nil, objCopy, &model.ReplaceIfExistingOption{})
		// make sure the workload is updated after the env CM
		cli.DependOn(dag, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: synthesizedComp.Namespace,
				Name:      constant.GenerateClusterComponentEnvPattern(synthesizedComp.ClusterName, synthesizedComp.Name),
			},
		})
	}
	return nil
}

func (t *componentWorkloadTransformer) handleWorkloadStartNStop(transCtx *componentTransformContext, synthesizedComp *component.SynthesizedComponent,
	runningITS *workloads.InstanceSet, protoITS **workloads.InstanceSet) (bool, bool) {
	var (
		stop  = isCompStopped(synthesizedComp)
		start = !stop && ptr.Deref(runningITS.Spec.Stop, false)
	)
	if start || stop {
		runningITSCopy := runningITS.DeepCopy() // don't modify the runningITS except for the stop flag
		runningITSCopy.Annotations[constant.KubeBlocksGenerationKey] = (*protoITS).Annotations[constant.KubeBlocksGenerationKey]
		*protoITS = runningITSCopy
	}
	if stop && checkPostProvisionDone(transCtx) {
		(*protoITS).Spec.Stop = ptr.To(true)
	}
	if start {
		(*protoITS).Spec.Stop = nil
	}
	return start, stop
}

func isCompStopped(synthesizedComp *component.SynthesizedComponent) bool {
	return ptr.Deref(synthesizedComp.Stop, false)
}

func (t *componentWorkloadTransformer) handleWorkloadUpdate(transCtx *componentTransformContext, dag *graph.DAG,
	synthesizeComp *component.SynthesizedComponent, comp *appsv1.Component, obj, its *workloads.InstanceSet) error {
	cwo, err := newComponentWorkloadOps(transCtx, t.Client, synthesizeComp, comp, obj, its, dag)
	if err != nil {
		return err
	}
	if err := cwo.horizontalScale(); err != nil {
		return err
	}
	return nil
}

// copyAndMergeITS merges two ITS objects for updating:
//  1. new an object targetObj by copying from oldObj
//  2. merge all fields can be updated from newObj into targetObj
func copyAndMergeITS(oldITS, newITS *workloads.InstanceSet, legacyConfigManagerPolicy legacyConfigManagerPolicy) *workloads.InstanceSet {
	itsObjCopy := oldITS.DeepCopy()
	itsProto := newITS

	// If the service version and component definition are not updated, we should not update the images in workload.
	checkNRollbackProtoImages(itsObjCopy, itsProto)

	// remove original monitor annotations
	if len(itsObjCopy.Annotations) > 0 {
		maps.DeleteFunc(itsObjCopy.Annotations, func(k, v string) bool {
			return strings.HasPrefix(k, "monitor.kubeblocks.io")
		})
	}
	intctrlutil.MergeMetadataMapInplace(itsProto.Annotations, &itsObjCopy.Annotations)
	intctrlutil.MergeMetadataMapInplace(itsProto.Labels, &itsObjCopy.Labels)
	// merge pod spec template annotations
	intctrlutil.MergeMetadataMapInplace(itsProto.Spec.Template.Annotations, &itsObjCopy.Spec.Template.Annotations)
	podTemplateCopy := *itsProto.Spec.Template.DeepCopy()
	podTemplateCopy.Annotations = itsObjCopy.Spec.Template.Annotations

	itsObjCopy.Spec.Template = podTemplateCopy
	// Preserve the legacy config-manager only for existing workloads that still have it in their live template.
	// This avoids an upgrade-only template diff from forcing all old Pods to restart after config-manager moved to kbagent.
	preserveLegacyConfigManagerPodSpec(oldITS, itsProto, itsObjCopy, legacyConfigManagerPolicy)
	itsObjCopy.Spec.Replicas = itsProto.Spec.Replicas
	itsObjCopy.Spec.Roles = itsProto.Spec.Roles
	itsObjCopy.Spec.LifecycleActions = itsProto.Spec.LifecycleActions
	itsObjCopy.Spec.Ordinals = itsProto.Spec.Ordinals
	itsObjCopy.Spec.Instances = itsProto.Spec.Instances
	itsObjCopy.Spec.FlatInstanceOrdinal = itsProto.Spec.FlatInstanceOrdinal
	itsObjCopy.Spec.OfflineInstances = itsProto.Spec.OfflineInstances
	itsObjCopy.Spec.MinReadySeconds = itsProto.Spec.MinReadySeconds
	itsObjCopy.Spec.VolumeClaimTemplates = itsProto.Spec.VolumeClaimTemplates
	itsObjCopy.Spec.PersistentVolumeClaimRetentionPolicy = itsProto.Spec.PersistentVolumeClaimRetentionPolicy
	itsObjCopy.Spec.ParallelPodManagementConcurrency = itsProto.Spec.ParallelPodManagementConcurrency
	itsObjCopy.Spec.PodUpdatePolicy = itsProto.Spec.PodUpdatePolicy
	itsObjCopy.Spec.PodUpgradePolicy = itsProto.Spec.PodUpgradePolicy
	itsObjCopy.Spec.InstanceUpdateStrategy = itsProto.Spec.InstanceUpdateStrategy
	itsObjCopy.Spec.MemberUpdateStrategy = itsProto.Spec.MemberUpdateStrategy
	itsObjCopy.Spec.Paused = itsProto.Spec.Paused
	itsObjCopy.Spec.Stop = itsProto.Spec.Stop
	itsObjCopy.Spec.Configs = itsProto.Spec.Configs
	itsObjCopy.Spec.Selector = itsProto.Spec.Selector
	itsObjCopy.Spec.DisableDefaultHeadlessService = itsProto.Spec.DisableDefaultHeadlessService
	itsObjCopy.Spec.EnableInstanceAPI = itsProto.Spec.EnableInstanceAPI
	itsObjCopy.Spec.InstanceAssistantObjects = itsProto.Spec.InstanceAssistantObjects

	if itsObjCopy.Spec.InstanceUpdateStrategy != nil && itsObjCopy.Spec.InstanceUpdateStrategy.RollingUpdate != nil {
		// use oldITS because itsObjCopy has been overwritten
		if oldITS.Spec.InstanceUpdateStrategy != nil &&
			oldITS.Spec.InstanceUpdateStrategy.RollingUpdate != nil &&
			oldITS.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable == nil {
			// HACK: This field is alpha-level (since v1.24) and is only honored by servers that enable the
			// MaxUnavailableStatefulSet feature.
			// When we get a nil MaxUnavailable from k8s, we consider that the field is not supported by the server,
			// and set the MaxUnavailable as nil explicitly to avoid the workload been updated unexpectedly.
			// Ref: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#maximum-unavailable-pods
			itsObjCopy.Spec.InstanceUpdateStrategy.RollingUpdate.MaxUnavailable = nil
		}
	}

	intctrlutil.ResolvePodSpecDefaultFields(oldITS.Spec.Template.Spec, &itsObjCopy.Spec.Template.Spec)

	isSpecUpdated := !reflect.DeepEqual(&oldITS.Spec, &itsObjCopy.Spec)
	isLabelsUpdated := !reflect.DeepEqual(oldITS.Labels, itsObjCopy.Labels)
	isAnnotationsUpdated := !reflect.DeepEqual(oldITS.Annotations, itsObjCopy.Annotations)
	if !isSpecUpdated && !isLabelsUpdated && !isAnnotationsUpdated {
		return nil
	}
	return itsObjCopy
}

const (
	legacyConfigManagerContainerName   = "config-manager"
	legacyConfigManagerToolsInitName   = "install-config-manager-tool"
	legacyConfigManagerToolsVolumeName = "kb-tools"
)

type legacyConfigManagerPolicy string

const (
	legacyConfigManagerPolicyKeep    legacyConfigManagerPolicy = "keep"
	legacyConfigManagerPolicyCleanup legacyConfigManagerPolicy = "cleanup"
)

func preserveLegacyConfigManagerPodSpec(oldITS, desiredITS, mergedITS *workloads.InstanceSet, legacyPolicy legacyConfigManagerPolicy) {
	if oldITS == nil || desiredITS == nil || mergedITS == nil {
		return
	}
	oldSpec := &oldITS.Spec.Template.Spec
	newSpec := &mergedITS.Spec.Template.Spec
	_, oldCfg := intctrlutil.GetContainerByName(oldSpec.Containers, legacyConfigManagerContainerName)
	if oldCfg == nil {
		return
	}
	// The compatibility marker is owned by upstream parameters logic. Existing workloads should keep the
	// legacy config-manager unless upstream has explicitly marked cleanup as safe.
	if legacyPolicy == legacyConfigManagerPolicyCleanup && shouldCleanupLegacyConfigManager(oldITS, desiredITS) {
		return
	}
	newSpec.Containers = preserveLegacyContainerOrder(oldSpec.Containers, newSpec.Containers, legacyConfigManagerContainerName, oldCfg)

	var oldInit *corev1.Container
	if _, initContainer := intctrlutil.GetContainerByName(oldSpec.InitContainers, legacyConfigManagerToolsInitName); initContainer != nil {
		oldInit = initContainer
		newSpec.InitContainers = preserveLegacyContainerOrder(oldSpec.InitContainers, newSpec.InitContainers, legacyConfigManagerToolsInitName, initContainer)
	}

	volumeNames := collectVolumeNamesFromContainers(oldCfg, oldInit)
	newSpec.Volumes = preserveLegacyVolumeOrder(oldSpec.Volumes, newSpec.Volumes, volumeNames)
	if _, ok := volumeNames[legacyConfigManagerToolsVolumeName]; ok {
		mergeVolumeMountsByVolumeName(oldSpec.Containers, &newSpec.Containers, legacyConfigManagerToolsVolumeName)
		mergeVolumeMountsByVolumeName(oldSpec.InitContainers, &newSpec.InitContainers, legacyConfigManagerToolsVolumeName)
	}
}

func legacyConfigManagerRequired(comp *appsv1.Component) legacyConfigManagerPolicy {
	if comp == nil || comp.Annotations == nil {
		return legacyConfigManagerPolicyKeep
	}
	value, ok := comp.Annotations[constant.LegacyConfigManagerRequiredAnnotationKey]
	if !ok {
		return legacyConfigManagerPolicyKeep
	}
	if value == "false" {
		return legacyConfigManagerPolicyCleanup
	}
	return legacyConfigManagerPolicyKeep
}

func shouldCleanupLegacyConfigManager(oldITS, desiredITS *workloads.InstanceSet) bool {
	oldTemplate := stripLegacyConfigManagerPodTemplate(oldITS.Spec.Template)
	newTemplate := stripLegacyConfigManagerPodTemplate(desiredITS.Spec.Template)
	if reflect.DeepEqual(oldTemplate, newTemplate) {
		return false
	}
	// Container list or init container changes are evaluated by InstanceSet as pod upgrade changes.
	// Clean up only when the configured policy will recreate Pods. Otherwise we keep the live template aligned
	// with old Pods that still carry the legacy sidecar and avoid creating an extra rollout only for cleanup.
	if hasPodUpgradeTemplateChanges(oldTemplate, newTemplate) {
		return desiredITS.Spec.PodUpgradePolicy == appsv1.ReCreatePodUpdatePolicyType
	}
	// Resource-only updates may still be in-place when vertical scaling is enabled; otherwise they recreate Pods.
	if hasPodResourceChanges(oldTemplate.Spec, newTemplate.Spec) {
		return !viper.GetBool(constant.FeatureGateInPlacePodVerticalScaling)
	}
	// Annotation/label/toleration style changes follow the regular pod update policy.
	if hasPodUpdateTemplateChanges(oldTemplate, newTemplate) {
		return desiredITS.Spec.PodUpdatePolicy == appsv1.ReCreatePodUpdatePolicyType
	}
	return false
}

func stripLegacyConfigManagerPodTemplate(template corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	template = *template.DeepCopy()
	volumeNames := map[string]struct{}{}
	template.Spec.Containers = filterLegacyContainers(template.Spec.Containers, volumeNames, legacyConfigManagerContainerName)
	template.Spec.InitContainers = filterLegacyContainers(template.Spec.InitContainers, volumeNames, legacyConfigManagerToolsInitName)
	template.Spec.Containers = filterLegacyVolumeMounts(template.Spec.Containers, volumeNames)
	template.Spec.InitContainers = filterLegacyVolumeMounts(template.Spec.InitContainers, volumeNames)
	template.Spec.Volumes = filterLegacyVolumes(template.Spec.Volumes, volumeNames)
	return template
}

func hasPodUpgradeTemplateChanges(oldTemplate, newTemplate corev1.PodTemplateSpec) bool {
	oldTemplate = *oldTemplate.DeepCopy()
	newTemplate = *newTemplate.DeepCopy()
	oldTemplate.Annotations = nil
	oldTemplate.Labels = nil
	newTemplate.Annotations = nil
	newTemplate.Labels = nil
	oldTemplate.Spec.ActiveDeadlineSeconds = nil
	newTemplate.Spec.ActiveDeadlineSeconds = nil
	oldTemplate.Spec.Tolerations = nil
	newTemplate.Spec.Tolerations = nil
	clearContainerResources(oldTemplate.Spec.Containers)
	clearContainerResources(oldTemplate.Spec.InitContainers)
	clearContainerResources(newTemplate.Spec.Containers)
	clearContainerResources(newTemplate.Spec.InitContainers)
	return !reflect.DeepEqual(oldTemplate.Spec, newTemplate.Spec)
}

func hasPodResourceChanges(oldSpec, newSpec corev1.PodSpec) bool {
	return !reflect.DeepEqual(getContainerResources(oldSpec.Containers), getContainerResources(newSpec.Containers)) ||
		!reflect.DeepEqual(getContainerResources(oldSpec.InitContainers), getContainerResources(newSpec.InitContainers))
}

func hasPodUpdateTemplateChanges(oldTemplate, newTemplate corev1.PodTemplateSpec) bool {
	if !reflect.DeepEqual(oldTemplate.Annotations, newTemplate.Annotations) {
		return true
	}
	if !reflect.DeepEqual(oldTemplate.Labels, newTemplate.Labels) {
		return true
	}
	if !reflect.DeepEqual(oldTemplate.Spec.ActiveDeadlineSeconds, newTemplate.Spec.ActiveDeadlineSeconds) {
		return true
	}
	return !reflect.DeepEqual(oldTemplate.Spec.Tolerations, newTemplate.Spec.Tolerations)
}

func filterLegacyContainers(containers []corev1.Container, volumeNames map[string]struct{}, legacyContainerName string) []corev1.Container {
	filtered := make([]corev1.Container, 0, len(containers))
	for _, container := range containers {
		if container.Name == legacyContainerName {
			for _, mount := range container.VolumeMounts {
				volumeNames[mount.Name] = struct{}{}
			}
			continue
		}
		filtered = append(filtered, container)
	}
	return filtered
}

func filterLegacyVolumeMounts(containers []corev1.Container, volumeNames map[string]struct{}) []corev1.Container {
	for i := range containers {
		mounts := containers[i].VolumeMounts[:0]
		for _, mount := range containers[i].VolumeMounts {
			if _, ok := volumeNames[mount.Name]; ok {
				continue
			}
			mounts = append(mounts, mount)
		}
		containers[i].VolumeMounts = mounts
	}
	return containers
}

func filterLegacyVolumes(volumes []corev1.Volume, volumeNames map[string]struct{}) []corev1.Volume {
	filtered := make([]corev1.Volume, 0, len(volumes))
	for _, volume := range volumes {
		if _, ok := volumeNames[volume.Name]; ok {
			continue
		}
		filtered = append(filtered, volume)
	}
	return filtered
}

func clearContainerResources(containers []corev1.Container) {
	for i := range containers {
		containers[i].Resources = corev1.ResourceRequirements{}
	}
}

func getContainerResources(containers []corev1.Container) map[string]corev1.ResourceRequirements {
	resources := make(map[string]corev1.ResourceRequirements, len(containers))
	for _, container := range containers {
		resources[container.Name] = container.Resources
	}
	return resources
}

func collectVolumeNamesFromContainers(containers ...*corev1.Container) map[string]struct{} {
	volumeNames := map[string]struct{}{}
	for _, container := range containers {
		if container == nil {
			continue
		}
		for _, mount := range container.VolumeMounts {
			volumeNames[mount.Name] = struct{}{}
		}
	}
	return volumeNames
}

func preserveLegacyContainerOrder(oldContainers, newContainers []corev1.Container, legacyName string, legacyContainer *corev1.Container) []corev1.Container {
	if legacyContainer == nil {
		return newContainers
	}
	if _, container := intctrlutil.GetContainerByName(newContainers, legacyName); container != nil {
		return newContainers
	}

	newContainerMap := make(map[string]corev1.Container, len(newContainers))
	used := make(map[string]struct{}, len(newContainers))
	for _, container := range newContainers {
		newContainerMap[container.Name] = container
	}

	merged := make([]corev1.Container, 0, len(newContainers)+1)
	for _, oldContainer := range oldContainers {
		switch {
		case oldContainer.Name == legacyName:
			// Reinsert the legacy container at its historical position to avoid order-only PodSpec diffs.
			merged = append(merged, *legacyContainer.DeepCopy())
		case hasContainerByName(newContainerMap, oldContainer.Name):
			merged = append(merged, newContainerMap[oldContainer.Name])
			used[oldContainer.Name] = struct{}{}
		}
	}
	for _, container := range newContainers {
		if _, ok := used[container.Name]; ok {
			continue
		}
		merged = append(merged, container)
	}
	return merged
}

func preserveLegacyVolumeOrder(oldVolumes, newVolumes []corev1.Volume, legacyVolumeNames map[string]struct{}) []corev1.Volume {
	if len(legacyVolumeNames) == 0 {
		return newVolumes
	}
	newVolumeMap := make(map[string]corev1.Volume, len(newVolumes))
	used := make(map[string]struct{}, len(newVolumes))
	for _, volume := range newVolumes {
		newVolumeMap[volume.Name] = volume
	}

	merged := make([]corev1.Volume, 0, len(newVolumes)+len(legacyVolumeNames))
	for _, oldVolume := range oldVolumes {
		switch {
		case hasVolumeByName(legacyVolumeNames, oldVolume.Name):
			// Keep the original volume order for the same reason as containers: avoid restart-causing order diffs.
			merged = append(merged, oldVolume)
		case hasVolumeByNameMap(newVolumeMap, oldVolume.Name):
			merged = append(merged, newVolumeMap[oldVolume.Name])
			used[oldVolume.Name] = struct{}{}
		}
	}
	for _, volume := range newVolumes {
		if _, ok := used[volume.Name]; ok {
			continue
		}
		if hasVolumeByName(legacyVolumeNames, volume.Name) {
			continue
		}
		merged = append(merged, volume)
	}
	return merged
}

func mergeVolumeMountsByVolumeName(oldContainers []corev1.Container, newContainers *[]corev1.Container, volumeName string) {
	for i := range *newContainers {
		container := &(*newContainers)[i]
		_, oldContainer := intctrlutil.GetContainerByName(oldContainers, container.Name)
		if oldContainer == nil {
			continue
		}
		container.VolumeMounts = preserveLegacyVolumeMountOrder(oldContainer.VolumeMounts, container.VolumeMounts, volumeName)
	}
}

func preserveLegacyVolumeMountOrder(oldMounts, newMounts []corev1.VolumeMount, volumeName string) []corev1.VolumeMount {
	hasLegacyMount := false
	for _, mount := range oldMounts {
		if mount.Name == volumeName {
			hasLegacyMount = true
			break
		}
	}
	if !hasLegacyMount || hasVolumeMount(newMounts, volumeName) {
		return newMounts
	}

	newMountMap := make(map[string]corev1.VolumeMount, len(newMounts))
	used := make(map[string]struct{}, len(newMounts))
	for _, mount := range newMounts {
		newMountMap[volumeMountKey(mount)] = mount
	}

	merged := make([]corev1.VolumeMount, 0, len(newMounts)+1)
	for _, oldMount := range oldMounts {
		if oldMount.Name == volumeName {
			merged = append(merged, oldMount)
		} else {
			key := volumeMountKey(oldMount)
			if mount, ok := newMountMap[key]; ok {
				merged = append(merged, mount)
				used[key] = struct{}{}
			}
		}
	}
	for _, mount := range newMounts {
		key := volumeMountKey(mount)
		if _, ok := used[key]; ok {
			continue
		}
		merged = append(merged, mount)
	}
	return merged
}

func hasContainerByName(containers map[string]corev1.Container, name string) bool {
	_, ok := containers[name]
	return ok
}

func hasVolumeByName(names map[string]struct{}, name string) bool {
	_, ok := names[name]
	return ok
}

func hasVolumeByNameMap(volumes map[string]corev1.Volume, name string) bool {
	_, ok := volumes[name]
	return ok
}

func hasVolumeMount(mounts []corev1.VolumeMount, name string) bool {
	for _, mount := range mounts {
		if mount.Name == name {
			return true
		}
	}
	return false
}

func volumeMountKey(mount corev1.VolumeMount) string {
	return mount.Name + "|" + mount.MountPath + "|" + mount.SubPath
}

func checkNRollbackProtoImages(itsObj, itsProto *workloads.InstanceSet) {
	if itsObj.Annotations == nil || itsProto.Annotations == nil {
		return
	}

	annotationUpdated := func(key string) bool {
		using, ok1 := itsObj.Annotations[key]
		proto, ok2 := itsProto.Annotations[key]
		if !ok1 || !ok2 {
			return true
		}
		if len(using) == 0 || len(proto) == 0 {
			return true
		}
		return using != proto
	}

	compDefUpdated := func() bool {
		return annotationUpdated(constant.AppComponentLabelKey)
	}

	serviceVersionUpdated := func() bool {
		return annotationUpdated(constant.KBAppServiceVersionKey)
	}

	if compDefUpdated() || serviceVersionUpdated() {
		return
	}

	// otherwise, roll-back the images in proto
	images := make([]map[string]string, 2)
	for i, cc := range [][]corev1.Container{itsObj.Spec.Template.Spec.InitContainers, itsObj.Spec.Template.Spec.Containers} {
		images[i] = make(map[string]string)
		for _, c := range cc {
			// skip the kb-agent container
			if component.IsKBAgentContainer(&c) {
				continue
			}
			images[i][c.Name] = c.Image
		}
	}
	rollback := func(idx int, c *corev1.Container) {
		if image, ok := images[idx][c.Name]; ok {
			c.Image = image
		}
	}
	for i := range itsProto.Spec.Template.Spec.InitContainers {
		rollback(0, &itsProto.Spec.Template.Spec.InitContainers[i])
	}
	for i := range itsProto.Spec.Template.Spec.Containers {
		rollback(1, &itsProto.Spec.Template.Spec.Containers[i])
	}
}

func hasMemberJoinNDataActionDefined(lifecycleActions *appsv1.ComponentLifecycleActions) (bool, bool) {
	if lifecycleActions == nil {
		return false, false
	}
	hasActionDefined := func(actions []*appsv1.Action) bool {
		for _, action := range actions {
			if !action.Defined() {
				return false
			}
		}
		return true
	}
	return hasActionDefined([]*appsv1.Action{lifecycleActions.MemberJoin}),
		hasActionDefined([]*appsv1.Action{lifecycleActions.DataDump, lifecycleActions.DataLoad})
}
