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
	"maps"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kbappsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/common"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	"github.com/apecloud/kubeblocks/pkg/generics"
)

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
		AddLabelsInMap(constant.GetCompLabels(clusterName, compName)).
		AddAnnotations(constant.KubeBlocksGenerationKey, synthesizedComp.Generation).
		AddAnnotations(constant.CRDAPIVersionAnnotationKey, workloads.GroupVersion.String()).
		AddAnnotationsInMap(map[string]string{
			constant.AppComponentLabelKey:   compDefName,
			constant.KBAppServiceVersionKey: synthesizedComp.ServiceVersion,
		}).
		AddAnnotationsInMap(synthesizedComp.StaticAnnotations).
		AddAnnotationsInMap(synthesizedComp.AnnotationsInjectedToWorkload).
		SetTemplate(getPodTemplate(synthesizedComp)).
		SetSelectorMatchLabel(getPodTemplateLabels(synthesizedComp)).
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
		SetConfigs(synthesizedComp.Configs).
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

	setDefaultResourceLimits(itsObj)

	return itsObj, nil
}

func getPodTemplate(synthesizedComp *SynthesizedComponent) corev1.PodTemplateSpec {
	podBuilder := builder.NewPodBuilder("", "").
		// priority: static < dynamic < built-in
		AddLabelsInMap(synthesizedComp.StaticLabels).
		AddLabelsInMap(synthesizedComp.DynamicLabels).
		AddLabelsInMap(getPodTemplateLabels(synthesizedComp)).
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

func getPodTemplateLabels(synthesizedComp *SynthesizedComponent) map[string]string {
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

func getPodManagementPolicy(synthesizedComp *SynthesizedComponent) appsv1.PodManagementPolicyType {
	if synthesizedComp.PodManagementPolicy != nil {
		return *synthesizedComp.PodManagementPolicy
	}
	return appsv1.OrderedReadyPodManagement // default value
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

func setDefaultResourceLimits(its *workloads.InstanceSet) {
	for _, cc := range []*[]corev1.Container{&its.Spec.Template.Spec.Containers, &its.Spec.Template.Spec.InitContainers} {
		for i := range *cc {
			intctrlutil.InjectZeroResourcesLimitsIfEmpty(&(*cc)[i])
		}
	}
}

func ListOwnedWorkloads(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) ([]*workloads.InstanceSet, error) {
	return listWorkloads(ctx, cli, namespace, clusterName, compName)
}

func ListOwnedPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string,
	opts ...client.ListOption) ([]*corev1.Pod, error) {
	return listPods(ctx, cli, namespace, clusterName, compName, nil, opts...)
}

func ListOwnedPodsWithRole(ctx context.Context, cli client.Reader, namespace, clusterName, compName, role string,
	opts ...client.ListOption) ([]*corev1.Pod, error) {
	roleLabel := map[string]string{constant.RoleLabelKey: role}
	return listPods(ctx, cli, namespace, clusterName, compName, roleLabel, opts...)
}

func ListOwnedServices(ctx context.Context, cli client.Reader, namespace, clusterName, compName string,
	opts ...client.ListOption) ([]*corev1.Service, error) {
	labels := constant.GetCompLabels(clusterName, compName)
	return listObjWithLabelsInNamespace(ctx, cli, generics.ServiceSignature, namespace, labels, opts...)
}

// GetMinReadySeconds gets the underlying workload's minReadySeconds of the component.
func GetMinReadySeconds(ctx context.Context, cli client.Client, cluster kbappsv1.Cluster, compName string) (minReadySeconds int32, err error) {
	var its []*workloads.InstanceSet
	its, err = listWorkloads(ctx, cli, cluster.Namespace, cluster.Name, compName)
	if err != nil {
		return
	}
	if len(its) > 0 {
		minReadySeconds = its[0].Spec.MinReadySeconds
		return
	}
	return minReadySeconds, err
}

func listWorkloads(ctx context.Context, cli client.Reader, namespace, clusterName, compName string) ([]*workloads.InstanceSet, error) {
	labels := constant.GetCompLabels(clusterName, compName)
	return listObjWithLabelsInNamespace(ctx, cli, generics.InstanceSetSignature, namespace, labels)
}

func listPods(ctx context.Context, cli client.Reader, namespace, clusterName, compName string,
	labels map[string]string, opts ...client.ListOption) ([]*corev1.Pod, error) {
	if labels == nil {
		labels = constant.GetCompLabels(clusterName, compName)
	} else {
		maps.Copy(labels, constant.GetCompLabels(clusterName, compName))
	}
	if opts == nil {
		opts = make([]client.ListOption, 0)
	}
	opts = append(opts, inDataContext()) // TODO: pod
	return listObjWithLabelsInNamespace(ctx, cli, generics.PodSignature, namespace, labels, opts...)
}

func listObjWithLabelsInNamespace[T generics.Object, PT generics.PObject[T], L generics.ObjList[T], PL generics.PObjList[T, L]](
	ctx context.Context, cli client.Reader, _ func(T, PT, L, PL), namespace string, labels client.MatchingLabels, opts ...client.ListOption) ([]PT, error) {
	if opts == nil {
		opts = make([]client.ListOption, 0)
	}
	opts = append(opts, []client.ListOption{labels, client.InNamespace(namespace)}...)

	var objList L
	if err := cli.List(ctx, PL(&objList), opts...); err != nil {
		return nil, err
	}

	objs := make([]PT, 0)
	items := reflect.ValueOf(&objList).Elem().FieldByName("Items").Interface().([]T)
	for i := range items {
		objs = append(objs, &items[i])
	}
	return objs, nil
}

func GetCurrentPodNamesByITS(runningITS *workloads.InstanceSet) ([]string, error) {
	itsExt, err := instancetemplate.BuildInstanceSetExt(runningITS, nil)
	if err != nil {
		return nil, err
	}
	nameBuilder, err := instancetemplate.NewPodNameBuilder(itsExt, nil)
	if err != nil {
		return nil, err
	}
	return nameBuilder.GenerateAllInstanceNames()
}

func GetDesiredPodNamesByITS(runningITS, protoITS *workloads.InstanceSet) ([]string, error) {
	if runningITS != nil {
		protoITS = protoITS.DeepCopy()
		protoITS.Status.AssignedOrdinals = runningITS.Status.AssignedOrdinals
	}
	return GetCurrentPodNamesByITS(protoITS)
}

func generatePodNamesByComp(comp *kbappsv1.Component) ([]string, error) {
	instanceTemplates := func() []workloads.InstanceTemplate {
		if len(comp.Spec.Instances) == 0 {
			return nil
		}
		templates := make([]workloads.InstanceTemplate, len(comp.Spec.Instances))
		for i, tpl := range comp.Spec.Instances {
			templates[i] = workloads.InstanceTemplate{
				Name:     tpl.Name,
				Replicas: tpl.Replicas,
				Ordinals: tpl.Ordinals,
			}
		}
		return templates
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   comp.Namespace,
			Name:        comp.Name,
			Annotations: comp.Annotations,
		},
		Spec: workloads.InstanceSetSpec{
			Replicas:            &comp.Spec.Replicas,
			Instances:           instanceTemplates(),
			Ordinals:            comp.Spec.Ordinals,
			FlatInstanceOrdinal: comp.Spec.FlatInstanceOrdinal,
			OfflineInstances:    comp.Spec.OfflineInstances,
		},
	}
	return GetCurrentPodNamesByITS(its)
}
