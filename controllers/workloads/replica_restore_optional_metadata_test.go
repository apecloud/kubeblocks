/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package workloads

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloadsv1 "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/replicarestore"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestInstanceSetReplicaRestoreStaleOptionalMetadata(t *testing.T) {
	scheme := replicaRestoreTestScheme(t)
	group := "dataprotection.kubeblocks.io"
	intent := &appsv1.ReplicaRestoreProjection{
		StartOrdinal: 1,
		EndOrdinal:   2,
		Fingerprint:  "accepted",
		SourceRef: corev1.TypedObjectReference{
			APIGroup: &group,
			Kind:     "Backup",
			Name:     "backup",
		},
		Annotations: map[string]string{constant.RestoreSourceNameAnnotationKey: "backup"},
	}
	its := &workloadsv1.InstanceSet{
		TypeMeta: metav1.TypeMeta{APIVersion: workloadsv1.GroupVersion.String(), Kind: workloadsv1.InstanceSetKind},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "mysql",
			Namespace:  "default",
			UID:        types.UID("its-uid"),
			Generation: 1,
			Labels:     map[string]string{constant.KBAppComponentLabelKey: "mysql"},
		},
		Spec: workloadsv1.InstanceSetSpec{
			Replicas:       ptr.To[int32](2),
			ReplicaRestore: intent,
			Selector:       &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mysql"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "mysql"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "db"}}},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data"}}},
		},
		Status: workloadsv1.InstanceSetStatus{ObservedGeneration: 1},
	}
	labels := map[string]string{
		constant.AppManagedByLabelKey:        constant.AppName,
		"workloads.kubeblocks.io/managed-by": workloadsv1.InstanceSetKind,
		"workloads.kubeblocks.io/instance":   its.Name,
	}
	predecessor := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mysql-0",
			Namespace:   its.Namespace,
			Labels:      labels,
			Annotations: map[string]string{"keep": "pod"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Image: "existing"}}},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{
				Type:               corev1.PodReady,
				Status:             corev1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
			}},
		},
	}
	targetName := "mysql-1"
	pvcName := intctrlutil.ComposePVCName(its.Spec.VolumeClaimTemplates[0], its.Name, targetName)
	pvc := ordinaryInstanceSetPVC(its, pvcName, targetName)
	replicarestore.ApplyToPVC(pvc, intent, targetName)
	pvc.Annotations[constant.RestorePITRAnnotationKey] = "stale"
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore),
		Status: corev1.ConditionTrue,
	}}

	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&workloadsv1.InstanceSet{}).
		WithObjects(its, predecessor, pvc).
		Build()
	r := &InstanceSetReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	reconcileTwice(t, r, types.NamespacedName{Namespace: its.Namespace, Name: its.Name})

	got := &workloadsv1.InstanceSet{}
	getObject(t, cli, types.NamespacedName{Namespace: its.Namespace, Name: its.Name}, got)
	if got.Status.ReplicaRestore == nil || got.Status.ReplicaRestore.Phase != appsv1.ReplicaRestoreFailed ||
		!strings.Contains(got.Status.ReplicaRestore.Message, constant.RestorePITRAnnotationKey) {
		t.Fatalf("expected persisted Failed status for stale PITR metadata, got %#v", got.Status.ReplicaRestore)
	}
	assertObjectUnchanged(t, cli, predecessor)
	assertObjectUnchanged(t, cli, pvc)
	assertObjectMissing(t, cli, &corev1.Pod{}, types.NamespacedName{Namespace: its.Namespace, Name: targetName})
}
