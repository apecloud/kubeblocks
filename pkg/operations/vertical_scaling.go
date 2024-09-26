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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/instanceset"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type verticalScalingHandler struct{}

var _ OpsHandler = verticalScalingHandler{}

func init() {
	vsHandler := verticalScalingHandler{}
	verticalScalingBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may can repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		OpsHandler:        vsHandler,
		QueueByCluster:    true,
		CancelFunc:        vsHandler.Cancel,
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.VerticalScalingType, verticalScalingBehaviour)
}

// ActionStartedCondition the started condition when handle the vertical scaling request.
func (vs verticalScalingHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewVerticalScalingCondition(opsRes.OpsRequest), nil
}

// Action modifies cluster component resources according to
// the definition of opsRequest with spec.componentNames and spec.componentOps.verticalScaling
func (vs verticalScalingHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	applyVerticalScaling := func(compSpec *appsv1.ClusterComponentSpec, obj ComponentOpsInterface) error {
		verticalScaling := obj.(opsv1alpha1.VerticalScaling)
		if vs.verticalScalingComp(verticalScaling) {
			compSpec.Resources = verticalScaling.ResourceRequirements
		}
		for _, v := range verticalScaling.Instances {
			for i := range compSpec.Instances {
				if compSpec.Instances[i].Name == v.Name {
					compSpec.Instances[i].Resources = &v.ResourceRequirements
					break
				}
			}
		}
		return nil
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.VerticalScalingType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			for _, v := range earlierOps.Spec.VerticalScalingList {
				// abort the earlierOps if exists the same component.
				if _, ok := compOpsSet.componentOpsSet[v.ComponentName]; ok {
					return true, nil
				}
			}
			return false, nil
		}); err != nil {
		return err
	}
	if err := compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, applyVerticalScaling); err != nil {
		return err
	}
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for vertical scaling opsRequest.
func (vs verticalScalingHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	handleComponentStatusProgressForVS := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		verticalScaling := pgRes.compOps.(opsv1alpha1.VerticalScaling)
		if len(pgRes.clusterComponent.Instances) != 0 {
			// obtain the pods which should be updated.
			updatedPodSet := map[string]string{}
			insMap := map[string]int32{}
			workloadName := constant.GenerateWorkloadNamePattern(opsRes.Cluster.Name, pgRes.fullComponentName)
			templateReplicasCnt := int32(0)
			for _, template := range pgRes.clusterComponent.Instances {
				replicas := template.GetReplicas()
				insMap[template.Name] = replicas
				templateReplicasCnt += replicas
			}
			for _, ins := range verticalScaling.Instances {
				replicas, ok := insMap[ins.Name]
				if !ok {
					continue
				}
				templatePodNames, err := instanceset.GenerateInstanceNamesFromTemplate(workloadName, ins.Name, replicas, pgRes.clusterComponent.OfflineInstances, nil)
				if err != nil {
					return 0, 0, err
				}
				for _, podName := range templatePodNames {
					updatedPodSet[podName] = ins.Name
				}
				break
			}
			if vs.verticalScalingComp(verticalScaling) && templateReplicasCnt < pgRes.clusterComponent.Replicas {
				podNames, err := instanceset.GenerateInstanceNamesFromTemplate(workloadName, "", pgRes.clusterComponent.Replicas-templateReplicasCnt, pgRes.clusterComponent.OfflineInstances, nil)
				if err != nil {
					return 0, 0, err
				}
				for _, podName := range podNames {
					updatedPodSet[podName] = ""
				}
			} else {
				pgRes.noWaitComponentCompleted = true
			}
			pgRes.updatedPodSet = updatedPodSet
		}
		return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus, vs.podApplyCompOps)
	}
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "vertical scale", handleComponentStatusProgressForVS)
}

func (vs verticalScalingHandler) verticalScalingComp(verticalScaling opsv1alpha1.VerticalScaling) bool {
	return len(verticalScaling.Requests) != 0 || len(verticalScaling.Limits) != 0
}

func (vs verticalScalingHandler) podApplyCompOps(
	ops *opsv1alpha1.OpsRequest,
	pod *corev1.Pod,
	compOps ComponentOpsInterface,
	insTemplateName string) bool {
	verticalScaling := compOps.(opsv1alpha1.VerticalScaling)
	if ops.Spec.Cancel {
		lastCompConfiguration := ops.Status.LastConfiguration.Components[verticalScaling.ComponentName]
		verticalScaling.Requests = lastCompConfiguration.Requests
		verticalScaling.Limits = lastCompConfiguration.Limits
	}
	matchResources := func(podResources, vsResources corev1.ResourceRequirements) bool {
		if vsResources.Requests == nil {
			vsResources.Requests = corev1.ResourceList{}
		}
		for resName, resValue := range vsResources.Limits {
			requestResource := vsResources.Requests[resName]
			if requestResource.IsZero() {
				vsResources.Requests[resName] = resValue
			}
			if !resValue.Equal(podResources.Limits[resName]) {
				return false
			}
		}
		for resName, resValue := range vsResources.Requests {
			if !resValue.Equal(podResources.Requests[resName]) {
				return false
			}
		}
		return true
	}
	if insTemplateName == constant.EmptyInsTemplateName {
		return matchResources(pod.Spec.Containers[0].Resources, verticalScaling.ResourceRequirements)
	}
	for _, insTpl := range verticalScaling.Instances {
		if insTpl.Name == insTemplateName {
			return matchResources(pod.Spec.Containers[0].Resources, verticalScaling.ResourceRequirements)
		}
	}
	return false
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (vs verticalScalingHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	compOpsHelper.saveLastConfigurations(opsRes, func(compSpec appsv1.ClusterComponentSpec, comOps ComponentOpsInterface) opsv1alpha1.LastComponentConfiguration {
		verticalScaling := comOps.(opsv1alpha1.VerticalScaling)
		var instanceTemplates []appsv1.InstanceTemplate
		for _, vIns := range verticalScaling.Instances {
			for _, compIns := range compSpec.Instances {
				if vIns.Name != compIns.Name {
					continue
				}
				instanceTemplates = append(instanceTemplates, appsv1.InstanceTemplate{
					Name:      compIns.Name,
					Resources: compIns.Resources,
				})
				break
			}
		}
		return opsv1alpha1.LastComponentConfiguration{
			ResourceRequirements: compSpec.Resources,
			Instances:            instanceTemplates,
		}
	})
	return nil
}

// Cancel this function defines the cancel verticalScaling action.
func (vs verticalScalingHandler) Cancel(reqCxt intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	return compOpsHelper.cancelComponentOps(reqCxt.Ctx, cli, opsRes, func(lastConfig *opsv1alpha1.LastComponentConfiguration, comp *appsv1.ClusterComponentSpec) {
		comp.Resources = lastConfig.ResourceRequirements
		for _, lastIns := range lastConfig.Instances {
			for i := range comp.Instances {
				if comp.Instances[i].Name != lastIns.Name {
					continue
				}
				comp.Instances[i].Resources = lastIns.Resources
				break
			}
		}
	})
}
