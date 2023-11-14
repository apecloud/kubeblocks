/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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
	"encoding/json"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/rsm"
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

	preserveObjects := func() error {
		if len(toPreserveKinds) == 0 {
			return nil
		}

		objs, err := getClusterOwningNamespacedObjects(transCtx, *cluster, ml, toPreserveKinds)
		if err != nil {
			return err
		}
		// construct cluster spec JSON string
		clusterSpec := cluster.DeepCopy()
		clusterSpec.ObjectMeta = metav1.ObjectMeta{
			Name: cluster.GetName(),
			UID:  cluster.GetUID(),
		}
		clusterSpec.Status = appsv1alpha1.ClusterStatus{}
		b, err := json.Marshal(*clusterSpec)
		if err != nil {
			return err
		}
		clusterJSON := string(b)
		for _, o := range objs {
			origObj := o.DeepCopyObject().(client.Object)
			controllerutil.RemoveFinalizer(o, constant.DBClusterFinalizerName)
			ownerRefs := o.GetOwnerReferences()
			for i, ref := range ownerRefs {
				if ref.Kind != appsv1alpha1.ClusterKind ||
					!strings.Contains(ref.APIVersion, appsv1alpha1.GroupVersion.Group) {
					continue
				}
				ownerRefs = append(ownerRefs[:i], ownerRefs[i+1:]...)
				break
			}
			o.SetOwnerReferences(ownerRefs)
			annot := o.GetAnnotations()
			if annot == nil {
				annot = map[string]string{}
			}
			// annotated last-applied Cluster spec
			annot[constant.LastAppliedClusterAnnotationKey] = clusterJSON
			o.SetAnnotations(annot)
			graphCli.Update(dag, origObj, o)
		}
		return nil
	}
	// handle preserved objects update vertex
	if err := preserveObjects(); err != nil {
		return err
	}

	toDeleteObjs := func(objs clusterOwningObjects) []client.Object {
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
	namespacedObjs, err := getClusterOwningNamespacedObjects(transCtx, *cluster, ml, toDeleteNamespacedKinds)
	if err != nil {
		return err
	}
	delObjs := toDeleteObjs(namespacedObjs)

	// add non-namespaced objects deletion vertex
	nonNamespacedObjs, err := getClusterOwningNonNamespacedObjects(transCtx, *cluster, ml, toDeleteNonNamespacedKinds)
	if err != nil {
		return err
	}
	delObjs = append(delObjs, toDeleteObjs(nonNamespacedObjs)...)

	for _, o := range delObjs {
		if !rsm.IsOwnedByRsm(o) {
			graphCli.Delete(dag, o)
		}
	}
	// set cluster action to noop until all the sub-resources deleted
	if len(delObjs) == 0 {
		graphCli.Delete(dag, cluster)
	} else {
		graphCli.Status(dag, cluster, transCtx.Cluster)
		// requeue since pvc isn't owned by cluster, and deleting it won't trigger event
		return newRequeueError(time.Second*1, "not all sub-resources deleted")
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
		&workloads.ReplicatedStateMachineList{},
		&policyv1.PodDisruptionBudgetList{},
		&corev1.ServiceList{},
		&corev1.ServiceAccountList{},
		&rbacv1.RoleBindingList{},
		&dpv1alpha1.BackupPolicyList{},
		&dpv1alpha1.BackupScheduleList{},
		&dpv1alpha1.RestoreList{},
		&batchv1.JobList{},
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
