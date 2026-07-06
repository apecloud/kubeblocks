/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package component

import (
	"encoding/json"

	appsk8s "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	viper "github.com/apecloud/kubeblocks/pkg/viperx"
)

// BuildInstanceSet builds an InstanceSet object from SynthesizedComponent.
func BuildInstanceSet(synthesizedComp *SynthesizedComponent, compDef *kbappsv1.ComponentDefinition) (*workloads.InstanceSet, error) {
	var (
		compDefName = synthesizedComp.CompDefName
		namespace   = synthesizedComp.Namespace
		clusterName = synthesizedComp.ClusterName
		compName    = synthesizedComp.Name
	)

	itsName := constant.GenerateWorkloadNamePattern(clusterName, compName)
	itsBuilder := builder.NewInstanceSetBuilder(namespace, itsName).
		// priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName, synthesizedComp.Labels)).
		AddAnnotations(constant.KubeBlocksGenerationKey, synthesizedComp.Generation).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, workloads.GroupVersion.String()).
		AddAnnotationsInMap(map[string]string{
			constant.AppComponentLabelKey:   compDefName,
			constant.KBAppServiceVersionKey: synthesizedComp.ServiceVersion,
		}).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(getMonitorAnnotations(synthesizedComp, compDef)).
		AddAnnotationsInMap(synthesizedComp.AnnotationsInjectedToWorkload).
		SetTemplate(getTemplate(synthesizedComp)).
		SetSelectorMatchLabel(getTemplateLabels(synthesizedComp)).
		SetReplicas(synthesizedComp.Replicas).
		SetVolumeClaimTemplates(defaultVolumeClaimTemplates(synthesizedComp)...).
		SetPVCRetentionPolicy(&synthesizedComp.PVCRetentionPolicy).
		SetMinReadySeconds(synthesizedComp.MinReadySeconds).
		SetInstances(getInstanceTemplates(synthesizedComp)).
		SetOrdinals(synthesizedComp.Ordinals).
		SetFlatInstanceOrdinal(synthesizedComp.FlatInstanceOrdinal).
		SetOfflineInstances(synthesizedComp.OfflineInstances).
		SetRoles(synthesizedComp.Roles).
		SetPodManagementPolicy(getPodManagementPolicy(synthesizedComp)).
		SetParallelPodManagementConcurrency(getParallelPodManagementConcurrency(synthesizedComp)).
		SetPodUpdatePolicy(synthesizedComp.PodUpdatePolicy).
		SetPodUpgradePolicy(synthesizedComp.PodUpgradePolicy).
		SetInstanceUpdateStrategy(getInstanceUpdateStrategy(synthesizedComp)).
		SetMemberUpdateStrategy(getMemberUpdateStrategy(synthesizedComp)).
		SetLifecycleActions(synthesizedComp.LifecycleActions.ComponentLifecycleActions, synthesizedComp.TemplateVars).
		// SetStop(synthesizedComp.Stop).  # check handleWorkloadStartNStop
		SetEnableInstanceAPI(synthesizedComp.EnableInstanceAPI).
		SetInstanceAssistantObjects(synthesizedComp.InstanceAssistantObjects)
	if compDef != nil {
		itsBuilder.SetDisableDefaultHeadlessService(compDef.Spec.DisableDefaultHeadlessService)
	}

	if common.IsCompactMode(synthesizedComp.Annotations) {
		itsBuilder.AddAnnotations(constant.FeatureReconciliationInCompactModeAnnotationKey,
			synthesizedComp.Annotations[constant.FeatureReconciliationInCompactModeAnnotationKey])
	}

	itsObj := itsBuilder.GetObject()

	if err := setDefaultResourceLimits(itsObj); err != nil {
		return nil, err
	}

	return itsObj, nil
}

func getTemplate(synthesizedComp *SynthesizedComponent) corev1.PodTemplateSpec {
	podBuilder := builder.NewPodBuilder("", "").
		// priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(getTemplateLabels(synthesizedComp)).
		AddLabelsInMap(map[string]string{
			constant.AppComponentLabelKey:   synthesizedComp.CompDefName,
			constant.KBAppServiceVersionKey: synthesizedComp.ServiceVersion,
		}).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(synthesizedComp.DynamicAnnotations)
	return corev1.PodTemplateSpec{
		ObjectMeta: podBuilder.GetObject().ObjectMeta,
		Spec:       *synthesizedComp.PodSpec.DeepCopy(),
	}
}

func getTemplateLabels(synthesizedComp *SynthesizedComponent) map[string]string {
	labels := constant.GetCompLabels(synthesizedComp.ClusterName, synthesizedComp.Name, synthesizedComp.Labels)
	labels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseStable
	return labels
}

func defaultVolumeClaimTemplates(synthesizedComp *SynthesizedComponent) []corev1.PersistentVolumeClaim {
	return toPersistentVolumeClaims(synthesizedComp, synthesizedComp.VolumeClaimTemplates)
}

func toPersistentVolumeClaims(synthesizedComp *SynthesizedComponent, vcts []corev1.PersistentVolumeClaimTemplate) []corev1.PersistentVolumeClaim {
	pvc := func(vct corev1.PersistentVolumeClaimTemplate) corev1.PersistentVolumeClaim {
		return corev1.PersistentVolumeClaim{
			ObjectMeta: vct.ObjectMeta,
			Spec:       vct.Spec,
		}
	}
	var pvcs []corev1.PersistentVolumeClaim
	for _, vct := range vcts {
		// priority: static < dynamic < built-in
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.StaticLabels, &vct.ObjectMeta.Labels)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.StaticAnnotations, &vct.ObjectMeta.Annotations)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicLabels, &vct.ObjectMeta.Labels)
		intctrlutil.MergeMetadataMapInplace(synthesizedComp.DynamicAnnotations, &vct.ObjectMeta.Annotations)
		pvcs = append(pvcs, pvc(vct))
	}
	return pvcs
}

func getInstanceTemplates(synthesizedComp *SynthesizedComponent) []workloads.InstanceTemplate {
	instances := synthesizedComp.Instances
	if instances == nil {
		return nil
	}
	instanceTemplates := make([]workloads.InstanceTemplate, len(instances))
	for i, tpl := range instances {
		instanceTemplates[i] = workloads.InstanceTemplate{
			Name:                 instances[i].Name,
			Replicas:             instances[i].Replicas,
			Ordinals:             instances[i].Ordinals,
			Annotations:          instances[i].Annotations,
			Labels:               instances[i].Labels,
			SchedulingPolicy:     instances[i].SchedulingPolicy,
			Resources:            instances[i].Resources,
			Env:                  instances[i].Env,
			VolumeClaimTemplates: toPersistentVolumeClaims(synthesizedComp, intctrlutil.ToCoreV1PVCTs(instances[i].VolumeClaimTemplates)),
			Images:               synthesizedComp.InstanceImages[instances[i].Name],
		}
		if ptr.Deref(tpl.Canary, false) {
			if instanceTemplates[i].Labels == nil {
				instanceTemplates[i].Labels = map[string]string{}
			}
			instanceTemplates[i].Labels[constant.KBAppReleasePhaseKey] = constant.ReleasePhaseCanary
		}
	}
	return instanceTemplates
}

func getPodManagementPolicy(synthesizedComp *SynthesizedComponent) appsk8s.PodManagementPolicyType {
	if synthesizedComp.PodManagementPolicy != nil {
		return *synthesizedComp.PodManagementPolicy
	}
	return appsk8s.OrderedReadyPodManagement // default value
}

func getParallelPodManagementConcurrency(synthesizedComp *SynthesizedComponent) *intstr.IntOrString {
	if synthesizedComp.ParallelPodManagementConcurrency != nil {
		return synthesizedComp.ParallelPodManagementConcurrency
	}
	return &intstr.IntOrString{Type: intstr.String, StrVal: "100%"} // default value
}

func getInstanceUpdateStrategy(synthesizedComp *SynthesizedComponent) *workloads.InstanceUpdateStrategy {
	// TODO: on-delete if the member update strategy is not null?
	return synthesizedComp.InstanceUpdateStrategy
}

func getMemberUpdateStrategy(synthesizedComp *SynthesizedComponent) *workloads.MemberUpdateStrategy {
	if synthesizedComp.UpdateStrategy != nil {
		return (*workloads.MemberUpdateStrategy)(synthesizedComp.UpdateStrategy)
	}
	return ptr.To(workloads.SerialUpdateStrategy)
}

// getMonitorAnnotations returns the annotations for the monitor.
func getMonitorAnnotations(synthesizedComp *SynthesizedComponent, componentDef *kbappsv1.ComponentDefinition) map[string]string {
	if synthesizedComp.DisableExporter == nil || *synthesizedComp.DisableExporter || componentDef == nil {
		return nil
	}

	exporter := GetExporter(componentDef.Spec)
	if exporter == nil {
		return nil
	}

	// Node: If it is an old addon, containerName may be empty.
	container := getBuiltinContainer(synthesizedComp, exporter.ContainerName)
	if container == nil && exporter.ScrapePort == "" && exporter.TargetPort == nil {
		klog.Warningf("invalid exporter port and ignore for component: %s, componentDef: %s", synthesizedComp.Name, componentDef.Name)
		return nil
	}
	return instanceset.AddAnnotationScope(instanceset.HeadlessServiceScope, common.GetScrapeAnnotations(*exporter, container))
}

func getBuiltinContainer(synthesizedComp *SynthesizedComponent, containerName string) *corev1.Container {
	containers := synthesizedComp.PodSpec.Containers
	for i := range containers {
		if containers[i].Name == containerName {
			return &containers[i]
		}
	}
	return nil
}

func setDefaultResourceLimits(its *workloads.InstanceSet) error {
	clusterResources, err := getClusterDefaultResources()
	if err != nil {
		return err
	}
	for i := range its.Spec.Template.Spec.Containers {
		container := &its.Spec.Template.Spec.Containers[i]
		if i > 0 {
			setClusterDefaultResources(container, clusterResources)
			continue
		}
		intctrlutil.InjectZeroResourcesLimitsIfEmpty(container)
	}
	for i := range its.Spec.Template.Spec.InitContainers {
		setClusterDefaultResources(&its.Spec.Template.Spec.InitContainers[i], clusterResources)
	}
	return nil
}

type clusterDefaultResources struct {
	Zero     bool                `json:"zero,omitempty"`
	Requests corev1.ResourceList `json:"requests,omitempty"`
	Limits   corev1.ResourceList `json:"limits,omitempty"`
}

func getClusterDefaultResources() (clusterDefaultResources, error) {
	resources := clusterDefaultResources{}
	value := viper.GetString(constant.CfgKeyClusterDefaultResources)
	if value == "" {
		return resources, nil
	}
	if err := json.Unmarshal([]byte(value), &resources); err != nil {
		return clusterDefaultResources{}, err
	}
	return resources, nil
}

func setClusterDefaultResources(container *corev1.Container, resources clusterDefaultResources) {
	for _, name := range []corev1.ResourceName{corev1.ResourceCPU, corev1.ResourceMemory} {
		if hasClusterDefaultResource(resources, name) {
			completeResource(container, resources, name)
			continue
		}
		if resources.Zero {
			intctrlutil.InjectZeroResourceLimitIfEmpty(container, name)
		}
	}
}

func hasClusterDefaultResource(resources clusterDefaultResources, name corev1.ResourceName) bool {
	_, hasRequest := resources.Requests[name]
	_, hasLimit := resources.Limits[name]
	return hasRequest || hasLimit
}

func completeResource(container *corev1.Container, resources clusterDefaultResources, name corev1.ResourceName) {
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}
	if container.Resources.Limits == nil {
		container.Resources.Limits = corev1.ResourceList{}
	}

	request, hasRequest := container.Resources.Requests[name]
	limit, hasLimit := container.Resources.Limits[name]
	if hasRequest && hasLimit {
		return
	}
	if hasRequest {
		container.Resources.Limits[name] = request
		return
	}
	if hasLimit {
		container.Resources.Requests[name] = limit
		return
	}

	request, hasRequest = resources.Requests[name]
	limit, hasLimit = resources.Limits[name]
	if hasRequest && !hasLimit {
		limit = request
	} else if !hasRequest && hasLimit {
		request = limit
	}
	container.Resources.Requests[name] = request
	container.Resources.Limits[name] = limit
}
