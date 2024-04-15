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
	"reflect"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
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
	var toPreserveKinds []client.ObjectList
	switch cluster.Spec.TerminationPolicy {
	case appsv1alpha1.DoNotTerminate:
		transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeWarning, "DoNotTerminate",
			"spec.terminationPolicy %s is preventing deletion.", cluster.Spec.TerminationPolicy)
		return graph.ErrPrematureStop
	case appsv1alpha1.Halt:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForHalt()
		toPreserveKinds = haltPreserveKinds()
	case appsv1alpha1.Delete:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForDelete()
	case appsv1alpha1.WipeOut:
		toDeleteNamespacedKinds, toDeleteNonNamespacedKinds = kindsForWipeOut()
	}

	transCtx.EventRecorder.Eventf(cluster, corev1.EventTypeNormal, constant.ReasonDeletingCR, "Deleting %s: %s",
		strings.ToLower(cluster.GetObjectKind().GroupVersionKind().Kind), cluster.GetName())

	// list all objects owned by this cluster in cache, and delete them all
	// there is chance that objects leak occurs because of cache stale
	// ignore the problem currently
	// TODO: GC the leaked objects
	ml := getAppInstanceML(*cluster)

	// handle preserved objects update vertex
	if err := preserveClusterObjects(transCtx.Context, transCtx.Client, graphCli, dag, cluster, ml, toPreserveKinds); err != nil {
		return err
	}

	toDeleteObjs := func(objs owningObjects) []client.Object {
		var delObjs []client.Object
		for _, obj := range objs {
			// retain backup for data protection even if the cluster is wiped out.
			if strings.EqualFold(obj.GetLabels()[constant.BackupProtectionLabelKey], constant.BackupRetain) {
				continue
			}
			delObjs = append(delObjs, obj)
		}
		return delObjs
	}

	// add namespaced objects deletion vertex
	namespacedObjs, err := getOwningNamespacedObjects(transCtx.Context, transCtx.Client, cluster.Namespace, ml, toDeleteNamespacedKinds)
	if err != nil {
		return err
	}
	delObjs := toDeleteObjs(namespacedObjs)

	// add non-namespaced objects deletion vertex
	nonNamespacedObjs, err := getOwningNonNamespacedObjects(transCtx.Context, transCtx.Client, ml, toDeleteNonNamespacedKinds)
	if err != nil {
		return err
	}
	delObjs = append(delObjs, toDeleteObjs(nonNamespacedObjs)...)

	for _, o := range delObjs {
		// skip the objects owned by the component and rsm controller
		if shouldSkipObjOwnedByComp(o, *cluster) || rsm.IsOwnedByRsm(o) {
			continue
		}
		graphCli.Delete(dag, o)
	}
	// set cluster action to noop until all the sub-resources deleted
	if len(delObjs) == 0 {
		graphCli.Delete(dag, cluster)
	} else {
		graphCli.Status(dag, cluster, transCtx.Cluster)
		// requeue since pvc isn't owned by cluster, and deleting it won't trigger event
		return newRequeueError(time.Second*1, "not all sub-resources deleted")
	}

	// release the allocated host ports
	// TODO release ports if scale in the components
	// TODO release ports one by one without using prefix
	pm := intctrlutil.GetPortManager()
	for _, comp := range transCtx.Cluster.Spec.ComponentSpecs {
		if err = pm.ReleaseByPrefix(fmt.Sprintf("%s-%s", transCtx.Cluster.Name, comp.Name)); err != nil {
			return newRequeueError(time.Second*1, "release host ports failed")
		}
	}

	// fast return, that is stopping the plan.Build() stage and jump to plan.Execute() directly
	return graph.ErrPrematureStop
}

func haltPreserveKinds() []client.ObjectList {
	return []client.ObjectList{
		&corev1.PersistentVolumeClaimList{},
		&corev1.SecretList{},
		&corev1.ConfigMapList{},
	}
}

func kindsForDoNotTerminate() ([]client.ObjectList, []client.ObjectList) {
	return []client.ObjectList{}, []client.ObjectList{}
}

func kindsForHalt() ([]client.ObjectList, []client.ObjectList) {
	namespacedKinds, nonNamespacedKinds := kindsForDoNotTerminate()
	namespacedKindsPlus := []client.ObjectList{
		&appsv1alpha1.ComponentList{},
		&appsv1.StatefulSetList{},           // be compatible with 0.6 workloads.
		&policyv1.PodDisruptionBudgetList{}, // be compatible with 0.6 workloads.
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

// preserveClusterObjects preserves the objects owned by the cluster when the cluster is being deleted
func preserveClusterObjects(ctx context.Context, cli client.Reader, graphCli model.GraphClient, dag *graph.DAG,
	cluster *appsv1alpha1.Cluster, ml client.MatchingLabels, toPreserveKinds []client.ObjectList) error {
	return preserveObjects(ctx, cli, graphCli, dag, cluster, ml, toPreserveKinds, constant.DBClusterFinalizerName, constant.LastAppliedClusterAnnotationKey)
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
