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
	rbacv1 "k8s.io/api/rbac/v1"
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
type componentDeletionTransformer struct{}

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
	if err := component.ReconcileCompPreTerminate(reqCtx, transCtx.Client, graphCli, cluster, comp, dag); err != nil {
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeExpectedInProcess) {
			// waiting for the preTerminate action to be done, and watch the action finish event to trigger the next reconcile
			return nil
		}
		if intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeRequeue) {
			return newRequeueError(time.Second*1, "request to requeue the component pre-terminate action")
		}
		return err
	}

	// step3: delete the sub-resources
	compShortName, err := component.ShortName(cluster.Name, comp.Name)
	if err != nil {
		return err
	}
	ml := constant.GetComponentWellKnownLabels(cluster.Name, compShortName)

	compScaleIn, ok := comp.Annotations[constant.ComponentScaleInAnnotationKey]
	if ok && compScaleIn == trueVal {
		return t.handleCompDeleteWhenScaleIn(transCtx, graphCli, dag, comp, ml)
	}
	return t.handleCompDeleteWhenClusterDelete(transCtx, graphCli, dag, cluster, comp, ml)
}

// handleCompDeleteWhenScaleIn handles the component deletion when scale-in, this scenario will delete all the sub-resources owned by the component by default.
func (t *componentDeletionTransformer) handleCompDeleteWhenScaleIn(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, comp *appsv1alpha1.Component, matchLabels map[string]string) error {
	return t.deleteCompResources(transCtx, graphCli, dag, comp, matchLabels, kindsForCompWipeOut())
}

// handleCompDeleteWhenClusterDelete handles the component deletion when the cluster is being deleted, the sub-resources owned by the component depends on the cluster's TerminationPolicy.
func (t *componentDeletionTransformer) handleCompDeleteWhenClusterDelete(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, cluster *appsv1alpha1.Cluster, comp *appsv1alpha1.Component, matchLabels map[string]string) error {
	var (
		toPreserveKinds, toDeleteKinds []client.ObjectList
	)
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.Halt:
		toPreserveKinds = compOwnedPreserveKinds()
		toDeleteKinds = kindsForCompHalt()
	case appsv1alpha1.Delete:
		toDeleteKinds = kindsForCompDelete()
	case appsv1alpha1.WipeOut:
		toDeleteKinds = kindsForCompWipeOut()
	}

	if len(toPreserveKinds) > 0 {
		// preserve the objects owned by the component when the component is being deleted
		if err := preserveCompObjects(transCtx.Context, transCtx.Client, graphCli, dag, comp, matchLabels, toPreserveKinds); err != nil {
			return newRequeueError(requeueDuration, err.Error())
		}
	}
	return t.deleteCompResources(transCtx, graphCli, dag, comp, matchLabels, toDeleteKinds)
}

func (t *componentDeletionTransformer) deleteCompResources(transCtx *componentTransformContext, graphCli model.GraphClient,
	dag *graph.DAG, comp *appsv1alpha1.Component, matchLabels map[string]string, toDeleteKinds []client.ObjectList) error {

	snapshot, err := model.ReadCacheSnapshot(transCtx, comp, matchLabels, toDeleteKinds...)
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
	err = transCtx.Client.Get(transCtx.Context, types.NamespacedName{Name: clusterName, Namespace: comp.Namespace}, cluster)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to get cluster %s: %v", clusterName, err))
	}
	return cluster, nil
}

func compOwnedKinds() []client.ObjectList {
	return []client.ObjectList{
		&workloads.ReplicatedStateMachineList{},
		&corev1.ServiceList{},
		&corev1.ServiceAccountList{},
		&rbacv1.RoleBindingList{},
		&batchv1.JobList{},
	}
}

func compOwnedPreserveKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.PersistentVolumeClaimList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
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
	preserveKinds := compOwnedPreserveKinds()
	return append(haltKinds, preserveKinds...)
}

func kindsForCompWipeOut() []client.ObjectList {
	return kindsForCompDelete()
}

// preserveCompObjects preserves the objects owned by the component when the component is being deleted
func preserveCompObjects(ctx context.Context, cli client.Reader, graphCli model.GraphClient, dag *graph.DAG,
	comp *appsv1alpha1.Component, ml client.MatchingLabels, toPreserveKinds []client.ObjectList) error {
	return preserveObjects(ctx, cli, graphCli, dag, comp, ml, toPreserveKinds, constant.DBComponentFinalizerName, constant.LastAppliedClusterAnnotationKey)
}
