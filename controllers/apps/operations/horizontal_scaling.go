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
	"slices"
	"time"

	"golang.org/x/exp/maps"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
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
	if slices.Contains([]appsv1alpha1.ClusterPhase{appsv1alpha1.StoppedClusterPhase,
		appsv1alpha1.StoppingClusterPhase}, opsRes.Cluster.Status.Phase) {
		return intctrlutil.NewFatalError("please start the cluster before scaling the cluster horizontally")
	}
	compOpsSet := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []appsv1alpha1.OpsType{appsv1alpha1.HorizontalScalingType, appsv1alpha1.StartType},
		func(earlierOps *appsv1alpha1.OpsRequest) (bool, error) {
			if slices.Contains([]appsv1alpha1.OpsType{appsv1alpha1.StartType, appsv1alpha1.StopType}, earlierOps.Spec.Type) {
				return true, nil
			}
			for _, v := range earlierOps.Spec.HorizontalScalingList {
				compOps, ok := compOpsSet.componentOpsSet[v.ComponentName]
				if !ok {
					return false, nil
				}
				currHorizontalScaling := compOps.(appsv1alpha1.HorizontalScaling)
				// When an earlier opsRequest has overlapping components with the current opsRequest,
				// and this operation for the component is an overwrite operation, it needs to be aborted.
				if v.Operator == appsv1alpha1.HScaleOverwriteOP || currHorizontalScaling.Operator == appsv1alpha1.HScaleOverwriteOP {
					return true, nil
				}
				// if the earlier opsRequest is pending and not `Overwrite` operator, return false.
				if earlierOps.Status.Phase == appsv1alpha1.OpsPendingPhase {
					return false, nil
				}
				if v.Operator != currHorizontalScaling.Operator {
					errMsg := fmt.Sprintf(`opsRequests only can run together with the same operator. `+
						`However, the opsRequest "%s" is currently running using the "%s" operator`,
						earlierOps.Name, v.Operator)
					return false, intctrlutil.NewFatalError(errMsg)
				}
				// check if the instance that needs to be offline was created by another opsRequest
				if currHorizontalScaling.Operator == appsv1alpha1.HScaleAddOP && len(currHorizontalScaling.OfflineInstances) > 0 {
					// TODO: check for sharding spec?
					lastCompConfiguration := earlierOps.Status.LastConfiguration.Components[v.ComponentName]
					compSpec := opsRes.Cluster.Spec.GetComponentByName(v.ComponentName)
					createdPodSet, _ := hs.getCreateAndDeletePodSet(opsRes, lastCompConfiguration, *compSpec, v, v.ComponentName)
					for _, offlineIns := range currHorizontalScaling.OfflineInstances {
						if _, ok = createdPodSet[offlineIns]; ok {
							errMsg := fmt.Sprintf(`instance "%s" cannot be taken offline as it has been created by another running opsRequest "%s"`,
								offlineIns, earlierOps.Name)
							return false, intctrlutil.NewFatalError(errMsg)
						}
					}
				}
			}
			return false, nil
		}); err != nil {
		return err
	}

	compOpsSet.updateClusterComponentsAndShardings(opsRes.Cluster, func(compSpec *appsv1alpha1.ClusterComponentSpec, obj ComponentOpsInteface) {
		horizontalScaling := obj.(appsv1alpha1.HorizontalScaling)
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[obj.GetComponentName()]
		compSpec.Replicas, compSpec.Instances, compSpec.OfflineInstances = hs.getExpectedCompValues(compSpec,
			lastCompConfiguration, horizontalScaling, opsRes.Cluster.Name)
	})
	return cli.Update(reqCtx.Ctx, opsRes.Cluster)
}

// getExpectedCompValues gets the expected replicas, instances, offlineInstances.
func (hs horizontalScalingOpsHandler) getExpectedCompValues(
	compSpec *appsv1alpha1.ClusterComponentSpec,
	lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
	horizontalScaling appsv1alpha1.HorizontalScaling,
	clusterName string) (int32, []appsv1alpha1.InstanceTemplate, []string) {
	compReplicas := compSpec.Replicas
	compInstanceTpls := compSpec.Instances
	compOfflineInstances := compSpec.OfflineInstances
	if horizontalScaling.Operator != appsv1alpha1.HScaleOverwriteOP {
		// `Add` and `Delete` operations require the use of recorded component snapshot information.
		compReplicas = *lastCompConfiguration.Replicas
		compInstanceTpls = lastCompConfiguration.Instances
		compOfflineInstances = lastCompConfiguration.OfflineInstances
	}
	expectOfflineInstances := hs.getCompExpectedOfflineInstances(compOfflineInstances, horizontalScaling)
	compReplicas, compInstanceTpls = hs.autoSyncReplicas(horizontalScaling, compReplicas,
		compInstanceTpls, compOfflineInstances, expectOfflineInstances, clusterName)
	return hs.getCompExpectReplicas(horizontalScaling, compReplicas),
		hs.getCompExpectedInstances(compInstanceTpls, horizontalScaling),
		expectOfflineInstances
}

// autoSyncReplicas auto-sync the replicas of the component and instance templates.
func (hs horizontalScalingOpsHandler) autoSyncReplicas(
	horizontalScaling appsv1alpha1.HorizontalScaling,
	compReplicas int32,
	compInstanceTpls []appsv1alpha1.InstanceTemplate,
	compOfflineInstances,
	expectCompOfflineInstances []string,
	clusterName string) (int32, []appsv1alpha1.InstanceTemplate) {
	if !horizontalScaling.AutoSyncReplicas {
		return compReplicas, compInstanceTpls
	}
	var (
		// pods will be added to offlineInstances slice.
		toAddOfflineInstances []string
		// pods will be deleted from offlineInstances slice.
		toDeleteOfflineInstances []string
	)
	switch horizontalScaling.Operator {
	case appsv1alpha1.HScaleAddOP:
		toAddOfflineInstances = horizontalScaling.OfflineInstances
	case appsv1alpha1.HScaleDeleteOP:
		toDeleteOfflineInstances = horizontalScaling.OfflineInstances
	default:
		currOfflineInsSet := sets.New(horizontalScaling.OfflineInstances...)
		oldOfflineInsSet := sets.New(compOfflineInstances...)
		toDeleteOfflineInstances = maps.Keys(oldOfflineInsSet.Difference(currOfflineInsSet))
		toAddOfflineInstances = maps.Keys(currOfflineInsSet.Difference(oldOfflineInsSet))
	}
	handleAutoSyncReplicas := func(podSet map[string]string, diffOfflineInstances []string, deleteOfflineInstance bool) {
		diffReplicasMap := map[string]int32{}
		for _, podName := range diffOfflineInstances {
			tplName, ok := podSet[podName]
			if !ok {
				continue
			}
			diffReplicasMap[tplName] += 1
			if deleteOfflineInstance {
				// scale out the instance which is removed fromm the offlineInstances slice.
				compReplicas += 1
			} else {
				compReplicas -= 1
			}
		}
		for i, insTpl := range compInstanceTpls {
			diffReplicas, ok := diffReplicasMap[insTpl.Name]
			if !ok {
				continue
			}
			if deleteOfflineInstance {
				compInstanceTpls[i].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(insTpl) + diffReplicas)
			} else {
				compInstanceTpls[i].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(insTpl) - diffReplicas)
			}
		}
	}

	if len(toDeleteOfflineInstances) > 0 {
		// scale out the specified instances by removing the instance from offlineInstances slice.
		podSet := getPodSetForComponent(compInstanceTpls, expectCompOfflineInstances,
			clusterName, horizontalScaling.ComponentName, compReplicas)
		handleAutoSyncReplicas(podSet, toDeleteOfflineInstances, true)
	}
	if len(toAddOfflineInstances) > 0 {
		// cale in the specified instances by adding the instance to offlineInstances slice.
		podSet := getPodSetForComponent(compInstanceTpls, compOfflineInstances,
			clusterName, horizontalScaling.ComponentName, compReplicas)
		handleAutoSyncReplicas(podSet, toAddOfflineInstances, false)
	}

	return compReplicas, compInstanceTpls
}

// getCompExpectReplicas gets the expected replicas of the component.
func (hs horizontalScalingOpsHandler) getCompExpectReplicas(horizontalScaling appsv1alpha1.HorizontalScaling,
	compSyncedReplicas int32) int32 {
	if horizontalScaling.Replicas == nil {
		return compSyncedReplicas
	}
	switch horizontalScaling.Operator {
	case appsv1alpha1.HScaleAddOP:
		return compSyncedReplicas + *horizontalScaling.Replicas
	case appsv1alpha1.HScaleDeleteOP:
		return compSyncedReplicas - *horizontalScaling.Replicas
	default:
		return *horizontalScaling.Replicas
	}
}

// getCompExpectedOfflineInstances gets the expected instance templates of the component.
func (hs horizontalScalingOpsHandler) getCompExpectedInstances(
	compSyncedInstanceTpls []appsv1alpha1.InstanceTemplate,
	horizontalScaling appsv1alpha1.HorizontalScaling,
) []appsv1alpha1.InstanceTemplate {
	if len(horizontalScaling.Instances) == 0 {
		// delete instance template is not supported, return directly.
		return compSyncedInstanceTpls
	}

	compInsTplSet := map[string]int{}
	for i := range compSyncedInstanceTpls {
		compInsTplSet[compSyncedInstanceTpls[i].Name] = i
	}
	for i := range horizontalScaling.Instances {
		insTpl := horizontalScaling.Instances[i]
		compInsIndex, ok := compInsTplSet[insTpl.Name]
		if !ok && horizontalScaling.Operator != appsv1alpha1.HScaleDeleteOP {
			// only support to add an instance template.
			compSyncedInstanceTpls = append(compSyncedInstanceTpls, insTpl)
			continue
		}
		if horizontalScaling.Instances[i].Replicas == nil {
			// ignore the changes if the replicas of the instance template is empty.
			continue
		}
		switch horizontalScaling.Operator {
		case appsv1alpha1.HScaleAddOP:
			compSyncedInstanceTpls[compInsIndex].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(compSyncedInstanceTpls[compInsIndex]) + *insTpl.Replicas)
		case appsv1alpha1.HScaleDeleteOP:
			compSyncedInstanceTpls[compInsIndex].Replicas = pointer.Int32(intctrlutil.TemplateReplicas(compSyncedInstanceTpls[compInsIndex]) - *insTpl.Replicas)
		default:
			compSyncedInstanceTpls[compInsIndex].Replicas = insTpl.Replicas
		}
	}
	return compSyncedInstanceTpls
}

// getCompExpectedOfflineInstances gets the expected offlineInstances of the component.
func (hs horizontalScalingOpsHandler) getCompExpectedOfflineInstances(
	compOfflineInstances []string,
	horizontalScaling appsv1alpha1.HorizontalScaling,
) []string {
	if len(horizontalScaling.OfflineInstances) == 0 {
		return compOfflineInstances
	}
	handleOfflineInstances := func(compOfflineInstances, hScaleOfflineInstances, newOfflineInstances []string) []string {
		compOfflineInsSet := sets.New(compOfflineInstances...)
		for _, offlineIns := range horizontalScaling.OfflineInstances {
			if _, ok := compOfflineInsSet[offlineIns]; !ok {
				newOfflineInstances = append(newOfflineInstances, offlineIns)
			}
		}
		return newOfflineInstances
	}
	switch horizontalScaling.Operator {
	case appsv1alpha1.HScaleAddOP:
		return handleOfflineInstances(compOfflineInstances, horizontalScaling.OfflineInstances, compOfflineInstances)
	case appsv1alpha1.HScaleDeleteOP:
		return handleOfflineInstances(compOfflineInstances, horizontalScaling.OfflineInstances, make([]string, 0))
	default:
		return horizontalScaling.OfflineInstances
	}
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for horizontal scaling opsRequest.
func (hs horizontalScalingOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (appsv1alpha1.OpsPhase, time.Duration, error) {
	handleComponentProgress := func(
		reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *appsv1alpha1.OpsRequestComponentStatus) (int32, int32, error) {
		lastCompConfiguration := opsRes.OpsRequest.Status.LastConfiguration.Components[pgRes.compOps.GetComponentName()]
		horizontalScaling := pgRes.compOps.(appsv1alpha1.HorizontalScaling)
		pgRes.createdPodSet, pgRes.deletedPodSet = hs.getCreateAndDeletePodSet(opsRes, lastCompConfiguration, *pgRes.clusterComponent, horizontalScaling, pgRes.fullComponentName)
		if horizontalScaling.Operator != appsv1alpha1.HScaleOverwriteOP {
			pgRes.noWaitComponentCompleted = true
		}
		return handleComponentProgressForScalingReplicas(reqCtx, cli, opsRes, pgRes, compStatus)
	}
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	return compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes, "", handleComponentProgress)
}

// SaveLastConfiguration records last configuration to the OpsRequest.status.lastConfiguration
func (hs horizontalScalingOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.HorizontalScalingList)
	getLastComponentInfo := func(compSpec appsv1alpha1.ClusterComponentSpec, comOps ComponentOpsInteface) appsv1alpha1.LastComponentConfiguration {
		lastCompConfiguration := appsv1alpha1.LastComponentConfiguration{
			Replicas:         pointer.Int32(compSpec.Replicas),
			Instances:        compSpec.Instances,
			OfflineInstances: compSpec.OfflineInstances,
		}
		return lastCompConfiguration
	}
	compOpsHelper.saveLastConfigurations(opsRes, getLastComponentInfo)
	return nil
}

// getCreateAndDeletePodSet gets the pod set that are created and deleted in this opsRequest.
func (hs horizontalScalingOpsHandler) getCreateAndDeletePodSet(opsRes *OpsResource,
	lastCompConfiguration appsv1alpha1.LastComponentConfiguration,
	currCompSpec appsv1alpha1.ClusterComponentSpec,
	horizontalScaling appsv1alpha1.HorizontalScaling,
	fullCompName string) (map[string]string, map[string]string) {
	clusterName := opsRes.Cluster.Name
	lastPodSet := getPodSetForComponent(lastCompConfiguration.Instances, lastCompConfiguration.OfflineInstances, clusterName, fullCompName, *lastCompConfiguration.Replicas)
	expectReplicas, expectInstanceTpls, expectOfflineInstances := hs.getExpectedCompValues(&currCompSpec, lastCompConfiguration, horizontalScaling, clusterName)
	currPodSet := getPodSetForComponent(expectInstanceTpls, expectOfflineInstances, clusterName, fullCompName, expectReplicas)
	createPodSet := map[string]string{}
	deletePodSet := map[string]string{}
	for k, v := range currPodSet {
		if _, ok := lastPodSet[k]; !ok {
			createPodSet[k] = v
		}
	}
	for k, v := range lastPodSet {
		if _, ok := currPodSet[k]; !ok {
			deletePodSet[k] = v
		}
	}
	if opsRes.OpsRequest.Status.Phase == appsv1alpha1.OpsCancellingPhase {
		// when cancelling this opsRequest, revert the changes.
		return deletePodSet, createPodSet
	}
	return createPodSet, deletePodSet
}

// Cancel this function defines the cancel horizontalScaling action.
func (hs horizontalScalingOpsHandler) Cancel(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.VerticalScalingList)
	return compOpsHelper.cancelComponentOps(reqCtx.Ctx, cli, opsRes, func(lastConfig *appsv1alpha1.LastComponentConfiguration, comp *appsv1alpha1.ClusterComponentSpec) {
		comp.Replicas = *lastConfig.Replicas
		comp.Instances = lastConfig.Instances
		comp.OfflineInstances = lastConfig.OfflineInstances
	})
}
