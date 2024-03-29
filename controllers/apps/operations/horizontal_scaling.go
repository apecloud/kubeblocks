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

package operations

import (
	"fmt"
	"math"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	intctrlcomp "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm2"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type horizontalScalingOpsHandler struct{}

var _ OpsHandler = horizontalScalingOpsHandler{}

func init() {
	hsHandler := horizontalScalingOpsHandler{}
	horizontalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1alpha1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1alpha1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        hsHandler,
		CancelFunc:        hsHandler.Cancel,
	}
	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(appsv1alpha1.HorizontalScalingType, horizontalScalingBehaviour)
}

// ActionStartedCondition the started condition when handling the horizontal scaling request.
func (hs horizontalScalingOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return appsv1alpha1.NewHorizontalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies Cluster.spec.components[*].replicas from the opsRequest
func (hs horizontalScalingOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	var (
		horizontalScalingMap = opsRes.OpsRequest.Spec.ToHorizontalScalingListToMap()
		horizontalScaling    appsv1alpha1.HorizontalScaling
		ok                   bool
	)
	for index, component := range opsRes.Cluster.Spec.ComponentSpecs {
		if horizontalScaling, ok = horizontalScalingMap[component.Name]; !ok {
			continue
		}
		r := horizontalScaling.Replicas
		opsRes.Cluster.Spec.ComponentSpecs[index].Replicas = r
		instances, err := buildInstances(opsRes.Cluster.Name, opsRes.Cluster.Spec.ComponentSpecs[index], horizontalScaling)
		if err != nil {
			return nil
		}
		opsRes.Cluster.Spec.ComponentSpecs[index].Instances = instances
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

type nameWithTemplate struct {
	instanceName string
	workloads.InstanceTemplate
}

// buildInstances constructs a new instances specification based on the current instances in ClusterComponentSpec and the instances to be updated (added/updated/deleted) in HorizontalScaling.
// For instances to be added:
// 1. The new instances are appended to the current instances.
// For instances to be updated:
// 1. A matching instance is searched for in the current instances based on the provided Name or GenerateName.
// 2. If a matching instance is found, it is updated. If necessary, the matching instance (template) is split.
// 3. An error is raised if no matching instance is found.
// For instances to be deleted:
// 1. A matching instance is searched for in the current instances based on the provided Name or GenerateName.
// 2. If a matching instance is found, it is deleted. If necessary, the matching instance (template) is split.
// 3. An error is raised if no matching instance is found.
// The newly constructed instances undergo validation, and an error is raised if they are invalid.
func buildInstances(clusterName string, componentSpec appsv1alpha1.ClusterComponentSpec, horizontalScaling appsv1alpha1.HorizontalScaling) ([]appsv1alpha1.InstanceTemplate, error) {
	if componentSpec.Instances == nil && horizontalScaling.InstancesToAdd == nil && horizontalScaling.InstancesToUpdate == nil && horizontalScaling.InstancesToDelete == nil {
		return nil, nil
	}
	getTotalReplicas := func(instances []appsv1alpha1.InstanceTemplate) (totalReplicas int32) {
		for _, instance := range instances {
			replicas := int32(1)
			if instance.Replicas != nil {
				replicas = *instance.Replicas
			}
			totalReplicas += replicas
		}
		return
	}
	componentName := intctrlcomp.FullName(clusterName, componentSpec.Name)
	name2TemplateMap := buildName2TemplateMap(componentName, componentSpec.Replicas, componentSpec.Instances)
	addedMap := buildName2TemplateMap(componentName, getTotalReplicas(horizontalScaling.InstancesToAdd), horizontalScaling.InstancesToAdd)
	for name, template := range addedMap {
		name2TemplateMap[name] = template
	}
	updatedMap := buildName2TemplateMap(componentName, getTotalReplicas(horizontalScaling.InstancesToUpdate), horizontalScaling.InstancesToUpdate)
	for name, template := range updatedMap {
		oldTemplate, ok := name2TemplateMap[name]
		if !ok {
			return nil, fmt.Errorf("no template for instance %s found to update", name)
		}
		name2TemplateMap[name] = mergeInstance(template, oldTemplate)
	}
	deletedMap := buildName2TemplateMap(componentName, getTotalReplicas(horizontalScaling.InstancesToDelete), horizontalScaling.InstancesToDelete)
	for name := range deletedMap {
		_, ok := name2TemplateMap[name]
		if !ok {
			return nil, fmt.Errorf("no template for instance %s found to delete", name)
		}
		delete(name2TemplateMap, name)
	}

	return rebuildInstanceTemplates(name2TemplateMap), nil
}

func rebuildInstanceTemplates(name2TemplateMap map[string]nameWithTemplate) []appsv1alpha1.InstanceTemplate {
	if len(name2TemplateMap) == 0 {
		return nil
	}
	var (
		instances         []appsv1alpha1.InstanceTemplate
		nameWithTemplates []nameWithTemplate
	)
	for _, template := range name2TemplateMap {
		nameWithTemplates = append(nameWithTemplates, template)
	}
	getNameNOrdinalFunc := func(i int) (string, int) {
		return rsm2.ParseParentNameAndOrdinal(nameWithTemplates[i].instanceName)
	}
	rsm2.BaseSort(nameWithTemplates, getNameNOrdinalFunc, nil, false)
	for i := range nameWithTemplates {
		if isHomogeneousInstance(nameWithTemplates, i-1, i) {
			*instances[len(instances)-1].Replicas++
			continue
		}
		instances = append(instances, *newInstanceTemplate(&nameWithTemplates[i]))
	}
	end := len(instances) - 1
	defaultInstance := appsv1alpha1.InstanceTemplate{Replicas: instances[end].Replicas}
	if reflect.DeepEqual(defaultInstance, instances[end]) {
		instances = instances[:end]
	}
	if len(instances) == 0 {
		instances = nil
	}
	return instances
}

func newInstanceTemplate(nameWithTemplate *nameWithTemplate) *appsv1alpha1.InstanceTemplate {
	instance := nameWithTemplate.InstanceTemplate
	replicas := int32(1)
	_, ordinal := rsm2.ParseParentNameAndOrdinal(nameWithTemplate.instanceName)
	var ordinalStart *int32
	if ordinal > 0 {
		ordinal32 := int32(ordinal)
		ordinalStart = &ordinal32
	}
	return &appsv1alpha1.InstanceTemplate{
		Replicas:             &replicas,
		Name:                 instance.Name,
		GenerateName:         instance.GenerateName,
		OrdinalStart:         ordinalStart,
		Annotations:          instance.Annotations,
		Labels:               instance.Labels,
		Image:                instance.Image,
		NodeName:             instance.NodeName,
		NodeSelector:         instance.NodeSelector,
		Tolerations:          instance.Tolerations,
		Resources:            instance.Resources,
		Volumes:              instance.Volumes,
		VolumeMounts:         instance.VolumeMounts,
		VolumeClaimTemplates: instance.VolumeClaimTemplates,
	}
}

func isHomogeneousInstance(nameWithTemplates []nameWithTemplate, i, j int) bool {
	if i < 0 || j < 0 {
		return false
	}
	// two instance names should be adjacent.
	pi, oi := rsm2.ParseParentNameAndOrdinal(nameWithTemplates[i].instanceName)
	pj, oj := rsm2.ParseParentNameAndOrdinal(nameWithTemplates[j].instanceName)
	if pi != pj || math.Abs(float64(oi-oj)) != 1 {
		return false
	}

	// same values
	setNilNameFields := func(t *workloads.InstanceTemplate) {
		t.Replicas = nil
		t.Name = nil
		t.GenerateName = nil
		t.OrdinalStart = nil
	}
	ti := nameWithTemplates[i].InstanceTemplate
	tj := nameWithTemplates[j].InstanceTemplate
	setNilNameFields(&ti)
	setNilNameFields(&tj)
	return reflect.DeepEqual(ti, tj)
}

func mergeInstance(src, dst nameWithTemplate) nameWithTemplate {
	if src.NodeName != nil {
		dst.NodeName = src.NodeName
	}
	rsm2.MergeMap(&src.Annotations, &dst.Annotations)
	rsm2.MergeMap(&src.Labels, &dst.Labels)
	rsm2.MergeMap(&src.NodeSelector, &dst.NodeSelector)
	if src.Image != nil {
		dst.Image = src.Image
	}
	rsm2.MergeList(&src.Volumes, &dst.Volumes,
		func(item corev1.Volume) func(corev1.Volume) bool {
			return func(v corev1.Volume) bool {
				return v.Name == item.Name
			}
		})
	rsm2.MergeList(&src.VolumeMounts, &dst.VolumeMounts,
		func(item corev1.VolumeMount) func(corev1.VolumeMount) bool {
			return func(vm corev1.VolumeMount) bool {
				return vm.Name == item.Name
			}
		})
	rsm2.MergeList(&src.VolumeClaimTemplates, &dst.VolumeClaimTemplates,
		func(item corev1.PersistentVolumeClaim) func(corev1.PersistentVolumeClaim) bool {
			return func(claim corev1.PersistentVolumeClaim) bool {
				return claim.Name == item.Name
			}
		})
	return dst
}

func buildName2TemplateMap(componentName string, replicas int32, instances []appsv1alpha1.InstanceTemplate) map[string]nameWithTemplate {
	// 1. build instance template groups
	var workloadsInstances []workloads.InstanceTemplate
	for _, instance := range instances {
		workloadsInstances = append(workloadsInstances, *intctrlcomp.AppsInstanceToWorkloadInstance(&instance))
	}
	instanceTemplateGroups := rsm2.BuildInstanceTemplateGroups(componentName, replicas, workloadsInstances, nil)

	// 2. build instance name to instance template map
	name2TemplateMap := make(map[string]nameWithTemplate, replicas)
	for parentName, templates := range instanceTemplateGroups {
		ordinal := 0
		for _, template := range templates {
			templateName := ""
			if template.Name != nil {
				templateName = *template.Name
			}
			generateName := parentName
			if template.GenerateName != nil {
				generateName = *template.GenerateName
			}
			ordinalStart := 0
			if template.OrdinalStart != nil {
				ordinalStart = int(*template.OrdinalStart)
			}
			replicas := 1
			if template.Replicas != nil {
				replicas = int(*template.Replicas)
			}
			name := ""
			for i := 0; i < replicas; i++ {
				name, ordinal = rsm2.GeneratePodName(templateName, generateName, ordinal, ordinalStart, i)
				name2TemplateMap[name] = nameWithTemplate{
					instanceName:     name,
					InstanceTemplate: *template,
				}
			}
		}
	}
	return name2TemplateMap
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus, hs.getExpectReplicas)
	}
	return reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", syncOverrideByOpsForScaleReplicas, handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	opsRequest := opsRes.OpsRequest
	lastComponentInfo := map[string]appsv1alpha1.LastComponentConfiguration{}
	componentNameMap := opsRequest.Spec.ToHorizontalScalingListToMap()
	for _, v := range opsRes.Cluster.Spec.ComponentSpecs {
		hsInfo, ok := componentNameMap[v.Name]
		if !ok {
			continue
		}
		copyReplicas := v.Replicas
		var copyInstances *[]appsv1alpha1.InstanceTemplate
		if len(v.Instances) > 0 {
			var instances []appsv1alpha1.InstanceTemplate
			instances = append(instances, v.Instances...)
			copyInstances = &instances
		}
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas:  &copyReplicas,
			Instances: copyInstances,
		}
		if hsInfo.Replicas < copyReplicas {
			podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, v.Name)
			if err != nil {
				return err
			}
			lastCompConfiguration.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{
				appsv1alpha1.PodsCompResourceKey: podNames,
			}
		}
		lastComponentInfo[v.Name] = lastCompConfiguration
	}
	opsRequest.Status.LastConfiguration.Components = lastComponentInfo
	return nil
}

func (hs horizontalScalingOpsHandler) getExpectReplicas(opsRequest *appsv1alpha1.OpsRequest, componentName string) *int32 {
	compStatus := opsRequest.Status.Components[componentName]
	if compStatus.OverrideBy != nil {
		return compStatus.OverrideBy.Replicas
	}
	for _, v := range opsRequest.Spec.HorizontalScalingList {
		if v.ComponentName == componentName {
			return &v.Replicas
		}
	}
	return nil
}

// getCompPodNamesBeforeScaleDownReplicas gets the component pod names before scale down replicas.
func getCompPodNamesBeforeScaleDownReplicas(reqCtx intctrlutil.RequestCtx,
	cli client.Client, cluster appsv1alpha1.Cluster, compName string) ([]string, error) {
	podNames := make([]string, 0)
	podList, err := intctrlcomp.GetComponentPodList(reqCtx.Ctx, cli, cluster, compName)
	if err != nil {
		return podNames, err
	}
	for _, v := range podList.Items {
		podNames = append(podNames, v.Name)
	}
	return podNames, nil
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	for _, v := range opsRes.OpsRequest.Status.Components {
		if v.OverrideBy != nil && v.OverrideBy.OpsName != "" {
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, `can not cancel the opsRequest due to another opsRequest "%s" is running`, v.OverrideBy.OpsName)
		}
	}
	return cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) error {
		if lastConfig.Replicas == nil {
			return nil
		}
		podNames, err := getCompPodNamesBeforeScaleDownReplicas(reqCtx, cli, *opsRes.Cluster, comp.Name)
		if err != nil {
			return err
		}
		if lastConfig.TargetResources == nil {
			lastConfig.TargetResources = map[appsv1alpha1.ComponentResourceKey][]string{}
		}
		lastConfig.TargetResources[appsv1alpha1.PodsCompResourceKey] = podNames
		comp.Replicas = *lastConfig.Replicas
		if lastConfig.Instances != nil {
			comp.Instances = *lastConfig.Instances
		}
		return nil
	})
}
