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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type restartOpsHandler struct {
	compOpsHelper componentOpsHelper
}

var _ OpsHandler = restartOpsHandler{}

func init() {
	restartBehaviour := OpsBehaviour{
		// if cluster is Abnormal or Failed, new opsRequest may repair it.
		FromClusterPhases: appsv1.GetClusterUpRunningPhases(),
		ToClusterPhase:    appsv1.UpdatingClusterPhase,
		QueueByCluster:    true,
		OpsHandler:        restartOpsHandler{},
	}

	opsMgr := GetOpsManager()
	opsMgr.RegisterOps(opsv1alpha1.RestartType, restartBehaviour)
}

// ActionStartedCondition the started condition when handle the restart request.
func (r restartOpsHandler) ActionStartedCondition(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (*metav1.Condition, error) {
	return opsv1alpha1.NewRestartingCondition(opsRes.OpsRequest), nil
}

// Action restarts components by updating StatefulSet.
func (r restartOpsHandler) Action(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	if opsRes.OpsRequest.Status.StartTimestamp.IsZero() {
		return fmt.Errorf("status.startTimestamp can not be null")
	}
	// abort earlier running vertical scaling opsRequest.
	if err := abortEarlierOpsRequestWithSameKind(reqCtx, cli, opsRes, []opsv1alpha1.OpsType{opsv1alpha1.RestartType},
		func(earlierOps *opsv1alpha1.OpsRequest) (bool, error) {
			return true, nil
		}); err != nil {
		return err
	}
	r.compOpsHelper = newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)
	orderedComps, err := r.getComponentOrders(reqCtx, cli, opsRes)
	if err != nil {
		return err
	}
	if len(orderedComps) > 0 {
		return r.restartComponents(reqCtx, cli, opsRes, orderedComps[:1], false)
	}
	return r.restartComponents(reqCtx, cli, opsRes, opsRes.OpsRequest.Spec.RestartList, false)
}

// ReconcileAction will be performed when action is done and loops till OpsRequest.status.phase is Succeed/Failed.
// the Reconcile function for restart opsRequest.
func (r restartOpsHandler) ReconcileAction(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) (opsv1alpha1.OpsPhase, time.Duration, error) {
	r.compOpsHelper := newComponentOpsHelper(opsRes.OpsRequest.Spec.RestartList)
	handleRestartProgress := func(reqCtx intctrlutil.RequestCtx,
		cli client.Client,
		opsRes *OpsResource,
		pgRes *progressResource,
		compStatus *opsv1alpha1.OpsRequestComponentStatus) (expectProgressCount int32, completedCount int32, err error) {
		return handleComponentStatusProgress(reqCtx, cli, opsRes, pgRes, compStatus, r.podApplyCompOps)
	}
	orderedComps, err := r.getComponentOrders(reqCtx, cli, opsRes)
	if err != nil {
		return "", 0, err
	}
	if len(orderedComps) > 1 {
		// the first component already restarts in "Action" step.
		if err := r.restartComponents(reqCtx, cli, opsRes, orderedComps, true); err != nil {
			return "", 0, err
		}
	}
	return r.compOpsHelper.reconcileActionWithComponentOps(reqCtx, cli, opsRes,
		"restart", handleRestartProgress)
}

// SaveLastConfiguration this operation only restart the pods of the component, no changes for Cluster.spec.
// empty implementation here.
func (r restartOpsHandler) SaveLastConfiguration(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) error {
	return nil
}

func (r restartOpsHandler) podApplyCompOps(
	ops *opsv1alpha1.OpsRequest,
	pod *corev1.Pod,
	compOps ComponentOpsInterface,
	insTemplateName string) bool {
	return !pod.CreationTimestamp.Before(&ops.Status.StartTimestamp)
}

func (r restartOpsHandler) getComponentOrders(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource) ([]opsv1alpha1.ComponentOps, error) {
	cd := &appsv1.ClusterDefinition{}
	if opsRes.Cluster.Spec.ClusterDefRef == "" || opsRes.Cluster.Spec.Topology == "" {
		return nil, nil
	}
	if err := cli.Get(reqCtx.Ctx, client.ObjectKey{Name: opsRes.Cluster.Spec.ClusterDefRef}, cd); err != nil {
		return nil, err
	}
	// components that require sequential restart
	var orderedComps []opsv1alpha1.ComponentOps
	for _, topology := range cd.Spec.Topologies {
		if topology.Name != opsRes.Cluster.Spec.Topology {
			continue
		}
		if topology.Orders != nil && len(topology.Orders.Update) > 0 {
			// when using clusterDef and topology, "update orders" includes all components
			for _, compName := range topology.Orders.Update {
				// get the ordered components to restart
				if compOps, ok := r.compOpsHelper.componentOpsSet[compName]; ok {
					orderedComps = append(orderedComps, compOps.(opsv1alpha1.ComponentOps))
				}
			}
		}
		break
	}
	return orderedComps, nil
}

func (r restartOpsHandler) restartComponents(reqCtx intctrlutil.RequestCtx, cli client.Client, opsRes *OpsResource, comOpsList []opsv1alpha1.ComponentOps, inOrder bool) error {
	for index, compOps := range comOpsList {
		if !r.matchToRestart(opsRes.OpsRequest, comOpsList, index, inOrder) {
			continue
		}
		matchingLabels := client.MatchingLabels{constant.AppInstanceLabelKey: opsRes.Cluster.Name}
		if opsRes.Cluster.Spec.GetShardingByName(compOps.ComponentName) != nil {
			matchingLabels[constant.KBAppShardingNameLabelKey] = compOps.ComponentName
		} else {
			matchingLabels[constant.KBAppComponentLabelKey] = compOps.ComponentName
		}
		instanceSetList := &workloads.InstanceSetList{}
		if err := cli.List(reqCtx.Ctx, instanceSetList,
			client.InNamespace(opsRes.Cluster.Namespace), matchingLabels); err != nil {
			return err
		}
		if len(instanceSetList.Items) == 0 {
			return fmt.Errorf(`the instanceSet workloads are not exists for the component "%s"`, compOps.ComponentName)
		}
		for i := range instanceSetList.Items {
			instanceSet := &instanceSetList.Items[i]
			if r.isRestarted(opsRes, instanceSet, &instanceSet.Spec.Template, compOps.ComponentName) {
				continue
			}
			if err := cli.Update(reqCtx.Ctx, instanceSet); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r restartOpsHandler) matchToRestart(opsRequest *opsv1alpha1.OpsRequest, comOpsList []opsv1alpha1.ComponentOps, index int, inOrder bool) bool {
	if !inOrder {
		return true
	}
	if index == 0 {
		// already restart in "Action" step.
		return false
	}
	compHasRestartCompleted := func(compName string) bool {
		progressDetails := opsRequest.Status.Components[compName].ProgressDetails
		for _, v := range progressDetails {
			if !isCompletedProgressStatus(v.Status) {
				return false
			}
		}
		return true
	}
	lastOrderCompName := comOpsList[index-1].ComponentName
	if !compHasRestartCompleted(lastOrderCompName) {
		return false
	}
	return !compHasRestartCompleted(comOpsList[index].ComponentName)
}

// isRestarted checks whether the component has been restarted
func (r restartOpsHandler) isRestarted(opsRes *OpsResource, object client.Object, podTemplate *corev1.PodTemplateSpec, componentName string) bool {
	cName := object.GetLabels()[constant.KBAppComponentLabelKey]
	shardingName := object.GetLabels()[constant.KBAppShardingNameLabelKey]
	if shardingName != "" {
		if _, ok := r.compOpsHelper.componentOpsSet[shardingName]; !ok {
			return true
		}
	} else {
		if _, ok := r.compOpsHelper.componentOpsSet[cName]; !ok {
			return true
		}
	}
	if podTemplate.Annotations == nil {
		podTemplate.Annotations = map[string]string{}
	}
	hasRestarted := true
	startTimestamp := opsRes.OpsRequest.Status.StartTimestamp
	workloadRestartTimeStamp := podTemplate.Annotations[constant.RestartAnnotationKey]
	if res, _ := time.Parse(time.RFC3339, workloadRestartTimeStamp); startTimestamp.After(res) {
		podTemplate.Annotations[constant.RestartAnnotationKey] = startTimestamp.Format(time.RFC3339)
		hasRestarted = false
	}
	return hasRestarted
}
