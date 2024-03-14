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

package apps

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// componentDeletionTransformer handles component deletion
type componentDeletionTransformer struct {
	client.Client
}

var _ graph.Transformer = &componentDeletionTransformer{}

func (t *componentDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*componentTransformContext)
	if transCtx.Component.GetDeletionTimestamp().IsZero() {
		return nil
	}

	reqCtx := intctrlutil.RequestCtx{
		Ctx:      transCtx.Context,
		Log:      transCtx.Logger,
		Recorder: transCtx.EventRecorder,
	}
	graphCli, _ := transCtx.Client.(model.GraphClient)
	comp := transCtx.Component
	cluster, err := t.getCluster(transCtx, comp)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}

	// step1: update the component status to deleting
	if comp.Status.Phase != appsv1alpha1.DeletingClusterCompPhase {
		comp.Status.Phase = appsv1alpha1.DeletingClusterCompPhase
		graphCli.Status(dag, comp, transCtx.Component)
		return newRequeueError(time.Second*1, "updating component status to deleting")
	}

	// step2: do the pre-terminate action if needed
	if err := component.ReconcileCompPreTerminate(reqCtx, t.Client, cluster, comp, dag); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorWaitCacheRefresh) {
			return newRequeueError(time.Second*3, "wait for preTerminate to be done")
		}
		return err
	}

	// step3: delete the sub-resources
	compShortName, err := component.ShortName(cluster.Name, comp.Name)
	if err != nil {
		return err
	}
	var (
		toDeleteKinds   []client.ObjectList
		toPreserveKinds []client.ObjectList
	)
	// by default, we inherit cluster termination policy to control the sub-resources deletion
	// TODO(xingran): check it is necessary to add a component-level terminatePolicy to control the sub-resources deletion
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate",
			"spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1alpha1.Halt:
		toDeleteKinds = kindsForCompHalt()
		toPreserveKinds = haltPreserveKinds()
	case appsv1alpha1.Delete:
		toDeleteKinds = kindsForCompDelete()
	case appsv1alpha1.WipeOut:
		toDeleteKinds = kindsForCompWipeOut()
	}

	// handle preserved objects update vertex
	ml := constant.GetComponentWellKnownLabels(cluster.Name, compShortName)
	if err := preserveCompObjects(transCtx.Context, transCtx.Client, graphCli, dag, comp, ml, toPreserveKinds); err != nil {
		return err
	}

	snapshot, err := model.ReadCacheSnapshot(transCtx, comp, ml, toDeleteKinds...)
	if err != nil {
		return newRequeueError(requeueDuration, err.Error())
	}
	if len(snapshot) > 0 {
		// delete the sub-resources owned by the component before deleting the component
		for _, object := range snapshot {
			if rsm.IsOwnedByRsm(object) {
				continue
			}
			graphCli.Delete(dag, object)
		}
		graphCli.Status(dag, comp, transCtx.Component)
		return newRequeueError(time.Second*1, "not all component sub-resources deleted")
	} else {
		graphCli.Delete(dag, comp)
	}

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func (t *componentDeletionTransformer) getCluster(transCtx *componentTransformContext, comp *appsv1alpha1.Component) (*appsv1alpha1.Cluster, error) {
	clusterName, err := component.GetClusterName(comp)
	if err != nil {
		return nil, err
	}
	cluster := &appsv1alpha1.Cluster{}
	err = t.Client.Get(transCtx.Context, types.NamespacedName{Name: clusterName, Namespace: comp.Namespace}, cluster)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get cluster %s: %v", clusterName, err))
	}
	return cluster, nil
}

func compOwnedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.ReplicatedStateMachineList{},
		&corev1.ServiceList{},
		&batchv1.JobList{},
	}
}

func kindsForCompDoNotTerminate() []client.ObjectList {
	return []client.ObjectList{}
}

func kindsForCompHalt() []client.ObjectList {
	doNotTerminateKinds := kindsForCompDoNotTerminate()
	ownedKinds := compOwnedKinds()
	return append(doNotTerminateKinds, ownedKinds...)
}

func kindsForCompDelete() []client.ObjectList {
	haltKinds := kindsForCompHalt()
	preserveKinds := haltPreserveKinds()
	return append(haltKinds, preserveKinds...)
}

func kindsForCompWipeOut() []client.ObjectList {
	return kindsForCompDelete()
}

// preserveCompObjects preserves the objects owned by the component when the component is being deleted
func preserveCompObjects(ctx context.Context, cli client.Reader, graphCli model.GraphClient, dag *graph.DAG,
	comp *appsv1alpha1.Component, ml client.MatchingLabels, toPreserveKinds []client.ObjectList) error {
	return preserveObjects(ctx, cli, graphCli, dag, comp, ml, toPreserveKinds, constant.DBComponentFinalizerName, constant.LastAppliedComponentAnnotationKey)
}
