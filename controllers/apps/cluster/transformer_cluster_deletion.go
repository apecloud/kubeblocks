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

package cluster

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	appsutil "github.com/apecloud/kubeblocks/controllers/apps/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// clusterDeletionTransformer handles cluster deletion
type clusterDeletionTransformer struct{}

var _ graph.Transformer = &clusterDeletionTransformer{}

func (t *clusterDeletionTransformer) Transform(ctx graph.TransformContext, dag *graph.DAG) error {
	transCtx, _ := ctx.(*clusterTransformContext)
	cluster := transCtx.OrigCluster
	if !cluster.IsDeleting() {
		return nil
	}

	graphCli, _ := transCtx.Client.(model.GraphClient)

	transCtx.Cluster.Status.Phase = appsv1.DeletingClusterPhase

	// list all kinds to be deleted based on v1alpha1.TerminationPolicyType
	var toDeleteNamespacedKinds, toDeleteNonNamespacedKinds []client.ObjectList
	switch cluster.Spec.TerminationPolicy {
	case appsv1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate",
			"spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1.Delete:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForDelete()
	case appsv1.WipeOut:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForWipeOut()
	}

	transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
		strings.ToLower(cluster.GetObjectKind().GroupVersionKind().Kind), cluster.GetName())

	// firstly, delete components and shardings in the order that topology defined.
	deleteSet, err := deleteCompNShardingInOrder4Terminate(transCtx, dag)
	if err != nil {
		return err
	}
	if len(deleteSet) > 0 {
		// wait for the components to be deleted to trigger the next reconcile
		transCtx.Logger.Info(fmt.Sprintf("wait for the components and shardings to be deleted: %v", deleteSet))
		return nil
	}

	// then list all the others objects owned by this cluster in cache, and delete them all
	ml := getAppInstanceML(*cluster)

	toDeleteObjs := func(objs owningObjects) []client.Object {
		var delObjs []client.Object
		for _, obj := range objs {
			if obj.GetObjectKind().GroupVersionKind().Kind == dptypes.BackupKind {
				backupObj := obj.(*dpv1alpha1.Backup)
				// retain backup for data protection even if the cluster is wiped out.
				if backupObj.Spec.DeletionPolicy == dpv1alpha1.BackupDeletionPolicyRetain {
					continue
				}
			}
			delObjs = append(delObjs, obj)
		}
		return delObjs
	}

	// add namespaced objects deletion vertex
	namespacedObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, cluster.Namespace, ml, toDeleteNamespacedKinds)
	if err != nil {
		// PDB or CRDs that not present in data-plane clusters
		if !strings.Contains(err.Error(), "the server could not find the requested resource") {
			return err
		}
	}
	if cluster.Spec.TerminationPolicy != appsv1.WipeOut {
		if err = getFailedBackups(transCtx.Context, transCtx.Client, cluster.Namespace, ml, namespacedObjs); err != nil {
			return err
		}
	}
	delObjs := toDeleteObjs(namespacedObjs)

	// add non-namespaced objects deletion vertex
	nonNamespacedObjs, err := getOwningNonNamespacedObjects(transCtx.Context, transCtx.Client, ml, toDeleteNonNamespacedKinds)
	if err != nil {
		// PDB or CRDs that not present in data-plane clusters
		if !strings.Contains(err.Error(), "the server could not find the requested resource") {
			return err
		}
	}
	delObjs = append(delObjs, toDeleteObjs(nonNamespacedObjs)...)

	delKindMap := map[string]sets.Empty{}
	for _, o := range delObjs {
		// skip the objects owned by the component and InstanceSet controller
		if isOwnedByComp(o) || appsutil.IsOwnedByInstanceSet(o) {
			continue
		}
		graphCli.Delete(dag, o, appsutil.InUniversalContext4G())
		delKindMap[o.GetObjectKind().GroupVersionKind().Kind] = sets.Empty{}
	}

	// set cluster action to status until all the sub-resources deleted
	if len(delObjs) == 0 {
		transCtx.Logger.Info(fmt.Sprintf("deleting cluster %v", klog.KObj(cluster)))
		graphCli.Delete(dag, cluster)
	} else {
		transCtx.Logger.Info(fmt.Sprintf("deleting the sub-resource kinds: %v", maps.Keys(delKindMap)))
		graphCli.Status(dag, cluster, transCtx.Cluster)
		// requeue since pvc isn't owned by cluster, and deleting it won't trigger event
		return intctrlutil.NewRequeueError(time.Second*1, "not all sub-resources deleted")
	}

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func kindsForDoNotTerminate() ([]client.ObjectList, []client.ObjectList) {
	return []client.ObjectList{}, []client.ObjectList{}
}

func kindsForDelete() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForDoNotTerminate()
	namespacedKindsPlus := []client.ObjectList{
		&appsv1.ComponentList{},
		&corev1.ServiceList{},
		&corev1.SecretList{},
		&dpv1alpha1.BackupPolicyList{},
		&dpv1alpha1.BackupScheduleList{},
	}
	return append(namespacedKinds, namespacedKindsPlus...), nonNamespacedKinds
}

func kindsForWipeOut() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForDelete()
	namespacedKindsPlus := []client.ObjectList{
		&dpv1alpha1.BackupList{},
	}
	return append(namespacedKinds, namespacedKindsPlus...), nonNamespacedKinds
}

func deleteCompNShardingInOrder4Terminate(transCtx *clusterTransformContext, dag *graph.DAG) (sets.Set[string], error) {
	nameSet, err := clusterRunningCompNShardingSet(transCtx.Context, transCtx.Client, transCtx.Cluster)
	if err != nil {
		return nil, err
	}
	if len(nameSet) == 0 {
		return nil, nil
	}
	if err = loadNCheckClusterDefinition(transCtx, transCtx.Cluster); err != nil {
		return nil, err
	}
	if err = deleteCompNShardingInOrder(transCtx, dag, nameSet, nil); err != nil {
		if intctrlutil.IsDelayedRequeueError(err) {
			delayedErr := err.(intctrlutil.DelayedRequeueError)
			err = intctrlutil.NewRequeueError(delayedErr.RequeueAfter(), delayedErr.Reason())
		}
		return nil, err
	}
	return nameSet, nil
}
