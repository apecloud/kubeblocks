/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package dataprotection

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func (r *VolumePopulatorReconciler) mapRestoreToPVCs(ctx context.Context, obj client.Object) []reconcile.Request {
	restore, ok := obj.(*dpv1alpha1.Restore)
	if !ok || restore.Labels[dprestore.DataProtectionRestoreLabelKey] != restore.Name {
		return nil
	}

	if owner := exactOwnerReference(restore.OwnerReferences, corev1.SchemeGroupVersion.String(), "PersistentVolumeClaim"); owner != nil {
		pvc := &corev1.PersistentVolumeClaim{}
		key := types.NamespacedName{Namespace: restore.Namespace, Name: owner.Name}
		if err := r.Client.Get(ctx, key, pvc); err != nil || pvc.UID != owner.UID ||
			!isClusterRestorePVC(pvc) || restore.Name != getPopulatePVCName(pvc.UID) {
			return nil
		}
		return []reconcile.Request{{NamespacedName: key}}
	}

	owner := exactOwnerReference(restore.OwnerReferences, appsv1.GroupVersion.String(), "Component")
	if owner == nil {
		return nil
	}
	comp := &appsv1.Component{}
	key := types.NamespacedName{Namespace: restore.Namespace, Name: owner.Name}
	if err := r.Client.Get(ctx, key, comp); err != nil || comp.UID != owner.UID ||
		restore.Name != postReadyRestoreName(comp.UID) {
		return nil
	}
	clusterName := comp.Labels[constant.AppInstanceLabelKey]
	componentName := restore.Labels[constant.KBAppComponentLabelKey]
	if clusterName == "" || componentName == "" || restore.Labels[constant.AppInstanceLabelKey] != clusterName {
		return nil
	}
	return r.mapRestorePVCs(ctx, restore.Namespace, client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
	})
}

func (r *VolumePopulatorReconciler) mapComponentToPVCs(ctx context.Context, obj client.Object) []reconcile.Request {
	comp, ok := obj.(*appsv1.Component)
	if !ok {
		return nil
	}
	clusterName := comp.Labels[constant.AppInstanceLabelKey]
	componentName := comp.Labels[constant.KBAppComponentLabelKey]
	if clusterName == "" || componentName == "" {
		return nil
	}
	return r.mapRestorePVCs(ctx, comp.Namespace, client.MatchingLabels{
		constant.AppInstanceLabelKey:    clusterName,
		constant.KBAppComponentLabelKey: componentName,
	})
}

func (r *VolumePopulatorReconciler) mapClusterToPVCs(ctx context.Context, obj client.Object) []reconcile.Request {
	cluster, ok := obj.(*appsv1.Cluster)
	if !ok || cluster.Name == "" {
		return nil
	}
	return r.mapRestorePVCs(ctx, cluster.Namespace, client.MatchingLabels{
		constant.AppInstanceLabelKey: cluster.Name,
	})
}

func (r *VolumePopulatorReconciler) mapRestorePVCs(ctx context.Context, namespace string,
	labels client.MatchingLabels) []reconcile.Request {
	list := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, list, client.InNamespace(namespace), labels); err != nil {
		return nil
	}
	requests := make([]reconcile.Request, 0, len(list.Items))
	for i := range list.Items {
		pvc := &list.Items[i]
		if !isClusterRestorePVC(pvc) || pvcRestoreTerminal(pvc) {
			continue
		}
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(pvc)})
	}
	return requests
}

func isClusterRestorePVC(pvc *corev1.PersistentVolumeClaim) bool {
	ref := pvc.Spec.DataSourceRef
	if ref == nil || ref.APIGroup == nil || *ref.APIGroup != dptypes.DataprotectionAPIGroup || ref.Name == "" ||
		(ref.Kind != dptypes.BackupKind && ref.Kind != dptypes.RestoreKind) {
		return false
	}
	if pvc.Labels[constant.AppInstanceLabelKey] == "" || pvc.Labels[constant.KBAppComponentLabelKey] == "" {
		return false
	}
	for _, key := range []string{
		constant.RestoreSourceAPIGroupAnnotationKey,
		constant.RestoreSourceKindAnnotationKey,
		constant.RestoreSourceNameAnnotationKey,
		constant.RestoreComponentAnnotationKey,
		constant.RestoreVolumeTemplateAnnotationKey,
	} {
		if pvc.Annotations[key] == "" {
			return false
		}
	}
	return pvc.Annotations[constant.RestoreComponentAnnotationKey] == pvc.Labels[constant.KBAppComponentLabelKey]
}

func pvcRestoreTerminal(pvc *corev1.PersistentVolumeClaim) bool {
	condition := findPVCConditionByType(pvc, appsv1.ConditionTypeRestore)
	return condition != nil && (condition.Status == corev1.ConditionTrue || condition.Status == corev1.ConditionFalse)
}

func exactOwnerReference(refs []metav1.OwnerReference, apiVersion, kind string) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].APIVersion == apiVersion && refs[i].Kind == kind && refs[i].Name != "" && refs[i].UID != "" {
			return &refs[i]
		}
	}
	return nil
}

func restoreDependencyPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldRestore, oldOK := e.ObjectOld.(*dpv1alpha1.Restore)
			newRestore, newOK := e.ObjectNew.(*dpv1alpha1.Restore)
			return oldOK && newOK && (oldRestore.Status.Phase != newRestore.Status.Phase ||
				!reflect.DeepEqual(oldRestore.DeletionTimestamp, newRestore.DeletionTimestamp))
		},
	}
}

func componentDependencyPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldComp, oldOK := e.ObjectOld.(*appsv1.Component)
			newComp, newOK := e.ObjectNew.(*appsv1.Component)
			return oldOK && newOK && (oldComp.Status.Phase != newComp.Status.Phase ||
				!reflect.DeepEqual(oldComp.DeletionTimestamp, newComp.DeletionTimestamp) ||
				!reflect.DeepEqual(postProvisionCondition(oldComp), postProvisionCondition(newComp)))
		},
	}
}

func clusterDependencyPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldCluster, oldOK := e.ObjectOld.(*appsv1.Cluster)
			newCluster, newOK := e.ObjectNew.(*appsv1.Cluster)
			return oldOK && newOK && (oldCluster.Status.Phase != newCluster.Status.Phase ||
				!reflect.DeepEqual(oldCluster.DeletionTimestamp, newCluster.DeletionTimestamp))
		},
	}
}

func postProvisionCondition(comp *appsv1.Component) *metav1.Condition {
	for i := range comp.Status.Conditions {
		condition := &comp.Status.Conditions[i]
		if condition.Type == appsv1.ComponentConditionProgressing && condition.Reason == "PostProvision" {
			return condition
		}
	}
	return nil
}
