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
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dprestore "github.com/apecloud/kubeblocks/pkg/dataprotection/restore"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestMapExecutionRestoreToTargetPVC(t *testing.T) {
	pvc := dependencyRestorePVC("data-mysql-0", "mysql", "pvc-uid")
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: pvc.Namespace,
		Name:      getPopulatePVCName(pvc.UID),
		Labels: map[string]string{
			dprestore.DataProtectionRestoreLabelKey: getPopulatePVCName(pvc.UID),
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "PersistentVolumeClaim",
			Name:       pvc.Name,
			UID:        pvc.UID,
		}},
	}}
	reconciler := dependencyTestReconciler(t, pvc)

	require.Equal(t, []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(pvc)}},
		reconciler.mapRestoreToPVCs(context.Background(), restore))

	badOwner := restore.DeepCopy()
	badOwner.OwnerReferences[0].UID = "another-pvc"
	require.Empty(t, reconciler.mapRestoreToPVCs(context.Background(), badOwner))
	sourceRestore := restore.DeepCopy()
	sourceRestore.Labels = nil
	require.Empty(t, reconciler.mapRestoreToPVCs(context.Background(), sourceRestore))
}

func TestMapPostReadyRestoreToNonTerminalComponentPVCs(t *testing.T) {
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster-mysql",
		UID:       "component-uid",
		Labels: map[string]string{
			constant.AppInstanceLabelKey:    "cluster",
			constant.KBAppComponentLabelKey: "mysql",
		},
	}}
	running := dependencyRestorePVC("data-mysql-0", "mysql", "running-pvc")
	terminal := dependencyRestorePVC("data-mysql-1", "mysql", "terminal-pvc")
	terminal.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type: corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore), Status: corev1.ConditionTrue,
	}}
	other := dependencyRestorePVC("data-postgresql-0", "postgresql", "other-pvc")
	restore := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{
		Namespace: comp.Namespace,
		Name:      postReadyRestoreName(comp.UID),
		Labels: map[string]string{
			dprestore.DataProtectionRestoreLabelKey: postReadyRestoreName(comp.UID),
			constant.AppInstanceLabelKey:            "cluster",
			constant.KBAppComponentLabelKey:         "mysql",
		},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.GroupVersion.String(), Kind: "Component", Name: comp.Name, UID: comp.UID,
		}},
	}}
	reconciler := dependencyTestReconciler(t, comp, running, terminal, other)

	require.Equal(t, []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(running)}},
		reconciler.mapRestoreToPVCs(context.Background(), restore))

	redirected := restore.DeepCopy()
	redirected.Labels[constant.KBAppComponentLabelKey] = "postgresql"
	require.Equal(t, []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(other)}},
		reconciler.mapRestoreToPVCs(context.Background(), redirected),
		"the Component owner validates the internal postReady Restore while labels identify the waiting PVCs")
}

func TestMapComponentAndClusterDependencies(t *testing.T) {
	mysql := dependencyRestorePVC("data-mysql-0", "mysql", "mysql-pvc")
	postgresql := dependencyRestorePVC("data-postgresql-0", "postgresql", "postgresql-pvc")
	invalid := dependencyRestorePVC("invalid", "mysql", "invalid-pvc")
	delete(invalid.Annotations, constant.RestoreSourceNameAnnotationKey)
	terminal := dependencyRestorePVC("terminal", "mysql", "terminal-pvc")
	terminal.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type: corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore), Status: corev1.ConditionFalse,
	}}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default", Name: "cluster-mysql",
		Labels: map[string]string{
			constant.AppInstanceLabelKey:    "cluster",
			constant.KBAppComponentLabelKey: "mysql",
		},
	}}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster"}}
	reconciler := dependencyTestReconciler(t, mysql, postgresql, invalid, terminal)

	require.Equal(t, []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(mysql)}},
		reconciler.mapComponentToPVCs(context.Background(), comp))
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: client.ObjectKeyFromObject(mysql)},
		{NamespacedName: client.ObjectKeyFromObject(postgresql)},
	}, reconciler.mapClusterToPVCs(context.Background(), cluster))
}

func TestDependencyPredicates(t *testing.T) {
	now := metav1.NewTime(time.Now())

	restoreOld := &dpv1alpha1.Restore{}
	restoreNew := restoreOld.DeepCopy()
	restoreNew.Status.Phase = dpv1alpha1.RestorePhaseRunning
	require.True(t, restoreDependencyPredicate().Update(event.UpdateEvent{ObjectOld: restoreOld, ObjectNew: restoreNew}))
	restoreUnrelated := restoreNew.DeepCopy()
	restoreUnrelated.Annotations = map[string]string{"example": "changed"}
	require.False(t, restoreDependencyPredicate().Update(event.UpdateEvent{ObjectOld: restoreNew, ObjectNew: restoreUnrelated}))
	restoreDeleting := restoreNew.DeepCopy()
	restoreDeleting.DeletionTimestamp = &now
	require.True(t, restoreDependencyPredicate().Update(event.UpdateEvent{ObjectOld: restoreNew, ObjectNew: restoreDeleting}))

	compOld := &appsv1.Component{}
	compNew := compOld.DeepCopy()
	compNew.Status.Conditions = []metav1.Condition{{
		Type: appsv1.ComponentConditionProgressing, Status: metav1.ConditionTrue, Reason: "PostProvision",
	}}
	require.True(t, componentDependencyPredicate().Update(event.UpdateEvent{ObjectOld: compOld, ObjectNew: compNew}))
	compUnrelated := compNew.DeepCopy()
	compUnrelated.Labels = map[string]string{"example": "changed"}
	require.False(t, componentDependencyPredicate().Update(event.UpdateEvent{ObjectOld: compNew, ObjectNew: compUnrelated}))
	compRunning := compNew.DeepCopy()
	compRunning.Status.Phase = appsv1.RunningComponentPhase
	require.True(t, componentDependencyPredicate().Update(event.UpdateEvent{ObjectOld: compNew, ObjectNew: compRunning}))

	clusterOld := &appsv1.Cluster{}
	clusterNew := clusterOld.DeepCopy()
	clusterNew.Status.Phase = appsv1.RunningClusterPhase
	require.True(t, clusterDependencyPredicate().Update(event.UpdateEvent{ObjectOld: clusterOld, ObjectNew: clusterNew}))
	clusterUnrelated := clusterNew.DeepCopy()
	clusterUnrelated.Labels = map[string]string{"example": "changed"}
	require.False(t, clusterDependencyPredicate().Update(event.UpdateEvent{ObjectOld: clusterNew, ObjectNew: clusterUnrelated}))
	clusterDeleting := clusterNew.DeepCopy()
	clusterDeleting.DeletionTimestamp = &now
	require.True(t, clusterDependencyPredicate().Update(event.UpdateEvent{ObjectOld: clusterNew, ObjectNew: clusterDeleting}))
}

func dependencyTestReconciler(t *testing.T, objects ...client.Object) *VolumePopulatorReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	return &VolumePopulatorReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build(),
		Scheme: scheme,
	}
}

func dependencyRestorePVC(name, componentName string, uid types.UID) *corev1.PersistentVolumeClaim {
	apiGroup := dptypes.DataprotectionAPIGroup
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default", Name: name, UID: uid,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    "cluster",
				constant.KBAppComponentLabelKey: componentName,
			},
			Annotations: map[string]string{
				constant.RestoreSourceAPIGroupAnnotationKey: dptypes.DataprotectionAPIGroup,
				constant.RestoreSourceKindAnnotationKey:     dptypes.BackupKind,
				constant.RestoreSourceNameAnnotationKey:     "backup",
				constant.RestoreComponentAnnotationKey:      componentName,
				constant.RestoreVolumeTemplateAnnotationKey: "data",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{DataSourceRef: &corev1.TypedObjectReference{
			APIGroup: &apiGroup, Kind: dptypes.BackupKind, Name: "backup",
		}},
	}
}
