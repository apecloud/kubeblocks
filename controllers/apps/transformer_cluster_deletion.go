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
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/exp/maps"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
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

	transCtx.Cluster.Status.Phase = appsv1alpha1.DeletingClusterPhase

	// list all kinds to be deleted based on v1alpha1.TerminationPolicyType
	var toDeleteNamespacedKinds, toDeleteNonNamespacedKinds []client.ObjectList
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate",
			"spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1alpha1.Halt:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "Halt",
			"spec.terminationPolicy %s is preventing deletion. Halt policy is deprecated is 0.9.1 and will have same meaning as DoNotTerminate.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1alpha1.Delete:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForDelete()
	case appsv1alpha1.WipeOut:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForWipeOut()
	}

	transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
		strings.ToLower(cluster.GetObjectKind().GroupVersionKind().Kind), cluster.GetName())

	// delete components in the order that topology defined.
	if err := deleteCompsInOrder4Terminate(transCtx, dag); err != nil {
		return err
	}

	// list all objects owned by this cluster in cache, and delete them all
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
	if cluster.Spec.TerminationPolicy != appsv1alpha1.WipeOut {
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
		if shouldSkipObjOwnedByComp(o, *cluster) || isOwnedByInstanceSet(o) {
			continue
		}
		// skip component objects since they are deleted in the previous step.
		if _, ok := o.(*appsv1alpha1.Component); ok {
			continue
		}
		graphCli.Delete(dag, o, inUniversalContext4G())
		delKindMap[o.GetObjectKind().GroupVersionKind().Kind] = sets.Empty{}
	}

	// set cluster action to noop until all the sub-resources deleted
	if len(delObjs) == 0 {
		graphCli.Delete(dag, cluster)
	} else {
		transCtx.Logger.Info(fmt.Sprintf("deleting the sub-resource kinds: %v", maps.Keys(delKindMap)))
		graphCli.Status(dag, cluster, transCtx.Cluster)
		// requeue since pvc isn't owned by cluster, and deleting it won't trigger event
		return newRequeueError(time.Second*1, "not all sub-resources deleted")
	}

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func kindsForDoNotTerminate() ([]client.ObjectList, []client.ObjectList) {
	return []client.ObjectList{}, []client.ObjectList{}
}

func kindsForHalt() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForDoNotTerminate()
	namespacedKindsPlus := []client.ObjectList{
		&appsv1alpha1.ComponentList{},
		&appsv1alpha1.OpsRequestList{},
		&appsv1.StatefulSetList{}, // be compatible with 0.6 workloads.
		&corev1.ServiceList{},
		&corev1.ServiceAccountList{}, // be backward compatible
		&rbacv1.RoleBindingList{},    // be backward compatible
		&dpv1alpha1.BackupPolicyList{},
		&dpv1alpha1.BackupScheduleList{},
		&dpv1alpha1.RestoreList{},
		&batchv1.JobList{},
		// The owner of the configuration in version 0.9 has been adjusted to component cr.
		// for compatible with version 0.8
		&appsv1alpha1.ConfigurationList{},
	}
	nonNamespacedKindsPlus := []client.ObjectList{
		&rbacv1.ClusterRoleBindingList{},
	}
	return append(namespacedKinds, namespacedKindsPlus...), append(nonNamespacedKinds, nonNamespacedKindsPlus...)
}

func kindsForDelete() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForHalt()
	return append(namespacedKinds, haltPreserveKinds()...), nonNamespacedKinds
}

func kindsForWipeOut() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForDelete()
	namespacedKindsPlus := []client.ObjectList{
		&dpv1alpha1.BackupList{},
	}
	return append(namespacedKinds, namespacedKindsPlus...), nonNamespacedKinds
}

// shouldSkipObjOwnedByComp is used to judge whether the object owned by component should be skipped when deleting the cluster
func shouldSkipObjOwnedByComp(obj client.Object, cluster appsv1alpha1.Cluster) bool {
	ownByComp := isOwnedByComp(obj)
	if !ownByComp {
		// if the object is not owned by component, it should not be skipped
		return false
	}

	// Due to compatibility reasons, the component controller creates cluster-scoped RoleBinding and ServiceAccount objects in the following two scenarios:
	// 1. When the user does not specify a ServiceAccount, KubeBlocks automatically creates a ServiceAccount and a RoleBinding with named pattern kb-{cluster.Name}.
	// 2. When the user specifies a ServiceAccount that does not exist, KubeBlocks will automatically create a ServiceAccount and a RoleBinding with the same name.
	// In both cases, the lifecycle of the RoleBinding and ServiceAccount should not be tied to the component. They should be deleted when the cluster is deleted.
	doNotSkipTypes := []interface{}{
		&rbacv1.RoleBinding{},
		&corev1.ServiceAccount{},
	}
	for _, t := range doNotSkipTypes {
		if objType, ok := obj.(interface{ GetName() string }); ok && reflect.TypeOf(obj) == reflect.TypeOf(t) {
			if strings.EqualFold(objType.GetName(), constant.GenerateDefaultServiceAccountName(cluster.GetName())) {
				return false
			}
			labels := obj.GetLabels()
			value, ok := labels[constant.AppManagedByLabelKey]
			if ok && value == constant.AppName {
				return false
			}
		}
	}
	return true
}

func deleteCompsInOrder4Terminate(transCtx *clusterTransformContext, dag *graph.DAG) error {
	compNameSet, err := component.GetClusterComponentShortNameSet(transCtx.Context, transCtx.Client, transCtx.Cluster)
	if err != nil {
		return err
	}
	if len(compNameSet) == 0 {
		return nil
	}
	if err = loadNCheckClusterDefinition(transCtx, transCtx.Cluster); err != nil {
		return err
	}
	return deleteCompsInOrder(transCtx, dag, compNameSet, true)
}
