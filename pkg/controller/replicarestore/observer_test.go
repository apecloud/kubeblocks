/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package replicarestore

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestObservePVCsCountsCompletedInstancesAcrossAllVCTs(t *testing.T) {
	intent := testIntent()
	targets := []Target{
		{InstanceName: "mysql-3", PVCs: []PVCIdentity{{Name: "data-mysql-3", VolumeClaimTemplate: "data"}, {Name: "scratch-mysql-3", VolumeClaimTemplate: "scratch"}}},
		{InstanceName: "mysql-4", PVCs: []PVCIdentity{{Name: "data-mysql-4", VolumeClaimTemplate: "data"}, {Name: "scratch-mysql-4", VolumeClaimTemplate: "scratch"}}},
	}
	pvcs := []*corev1.PersistentVolumeClaim{
		observedPVC(t, intent, "mysql-3", "data-mysql-3", "data", corev1.ConditionTrue),
		observedPVC(t, intent, "mysql-3", "scratch-mysql-3", "scratch", corev1.ConditionTrue),
		observedPVC(t, intent, "mysql-4", "data-mysql-4", "data", corev1.ConditionTrue),
		observedPVC(t, intent, "mysql-4", "scratch-mysql-4", "scratch", corev1.ConditionTrue),
	}

	status := ObservePVCs(intent, "mysql", 7, 5, 2, targets, pvcs)
	if status.Phase != appsv1.ReplicaRestoreCompleted || status.RestoredReplicas != 2 || status.TargetReplicas != 5 || status.ObservedGeneration != 7 {
		t.Fatalf("unexpected summary: %#v", status)
	}

	status = ObservePVCs(intent, "mysql", 7, 5, 2, targets, pvcs[:3])
	if status.Phase != appsv1.ReplicaRestorePending || status.RestoredReplicas != 1 {
		t.Fatalf("missing auxiliary PVC should leave one restored instance: %#v", status)
	}

	pvcs[3].Status.Conditions[0].Status = corev1.ConditionFalse
	status = ObservePVCs(intent, "mysql", 7, 5, 2, targets, pvcs)
	if status.Phase != appsv1.ReplicaRestoreFailed {
		t.Fatalf("failed auxiliary PVC should fail the restore: %#v", status)
	}
}

func TestObservePVCsFailsClosedOnIdentityMismatch(t *testing.T) {
	intent := testIntent()
	target := Target{InstanceName: "mysql-3", PVCs: []PVCIdentity{{Name: "data-mysql-3", VolumeClaimTemplate: "data"}}}
	tests := map[string]func(*corev1.PersistentVolumeClaim){
		"purpose": func(pvc *corev1.PersistentVolumeClaim) { delete(pvc.Annotations, constant.RestorePurposeAnnotationKey) },
		"source":  func(pvc *corev1.PersistentVolumeClaim) { pvc.Spec.DataSourceRef.Name = "other" },
		"fingerprint": func(pvc *corev1.PersistentVolumeClaim) {
			pvc.Annotations[constant.ReplicaRestoreFingerprintAnnotationKey] = "other"
		},
		"vct annotation": func(pvc *corev1.PersistentVolumeClaim) {
			pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = "logs"
		},
		"vct label": func(pvc *corev1.PersistentVolumeClaim) { pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] = "logs" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			pvc := observedPVC(t, intent, "mysql-3", "data-mysql-3", "data", corev1.ConditionTrue)
			mutate(pvc)
			status := ObservePVCs(intent, "mysql", 7, 5, 1, []Target{target}, []*corev1.PersistentVolumeClaim{pvc})
			if status.Phase != appsv1.ReplicaRestoreFailed {
				t.Fatalf("identity mismatch should fail closed: %#v", status)
			}
		})
	}
}

func TestObserveTargetConditionsRequiresFreshMatchingProjection(t *testing.T) {
	intent := testIntent()
	completed := metav1.Condition{Status: metav1.ConditionTrue, ObservedGeneration: 4}
	target := TargetCondition{Name: "mysql-3", Projection: intent.DeepCopy(), Generation: 4, ObservedGeneration: 4, Condition: &completed}
	status := ObserveTargetConditions(intent, "mysql", 8, 5, 1, []TargetCondition{target})
	if status.Phase != appsv1.ReplicaRestoreCompleted || status.RestoredReplicas != 1 {
		t.Fatalf("unexpected completed summary: %#v", status)
	}

	target.ObservedGeneration = 3
	status = ObserveTargetConditions(intent, "mysql", 8, 5, 1, []TargetCondition{target})
	if status.Phase != appsv1.ReplicaRestorePending {
		t.Fatalf("stale Instance status should remain pending: %#v", status)
	}

	target.ObservedGeneration = 4
	target.Projection = intent.DeepCopy()
	target.Projection.Fingerprint = "other"
	status = ObserveTargetConditions(intent, "mysql", 8, 5, 1, []TargetCondition{target})
	if status.Phase != appsv1.ReplicaRestorePending {
		t.Fatalf("projection propagation lag should remain pending: %#v", status)
	}
}

func TestObservePVCsDoesNotCompleteMissingAssignmentsOrEmptyVCTs(t *testing.T) {
	intent := testIntent()
	status := ObservePVCs(intent, "mysql", 7, 5, 2, []Target{{InstanceName: "mysql-3"}}, nil)
	if status.Phase != appsv1.ReplicaRestorePending || status.RestoredReplicas != 0 {
		t.Fatalf("missing target assignment and VCTs should remain pending: %#v", status)
	}
}

func TestObserversCountFullProgressAndChooseDeterministicFailure(t *testing.T) {
	intent := testIntent()
	intent.EndOrdinal = 6
	targets := []Target{
		{InstanceName: "mysql-5", PVCs: []PVCIdentity{{Name: "data-mysql-5", VolumeClaimTemplate: "data"}}},
		{InstanceName: "mysql-4", PVCs: []PVCIdentity{{Name: "data-mysql-4", VolumeClaimTemplate: "data"}}},
		{InstanceName: "mysql-3", PVCs: []PVCIdentity{{Name: "data-mysql-3", VolumeClaimTemplate: "data"}}},
	}
	pvcs := []*corev1.PersistentVolumeClaim{
		observedPVC(t, intent, "mysql-5", "data-mysql-5", "data", corev1.ConditionFalse),
		observedPVC(t, intent, "mysql-4", "data-mysql-4", "data", corev1.ConditionTrue),
		observedPVC(t, intent, "mysql-3", "data-mysql-3", "data", corev1.ConditionFalse),
	}
	status := ObservePVCs(intent, "mysql", 7, 6, 3, targets, pvcs)
	if status.Phase != appsv1.ReplicaRestoreFailed || status.RestoredReplicas != 1 || !strings.Contains(status.Message, "data-mysql-3") {
		t.Fatalf("unexpected deterministic PVC failure summary: %#v", status)
	}

	failed3 := metav1.Condition{Status: metav1.ConditionFalse, ObservedGeneration: 1, Message: "failed three"}
	failed5 := metav1.Condition{Status: metav1.ConditionFalse, ObservedGeneration: 1, Message: "failed five"}
	completed := metav1.Condition{Status: metav1.ConditionTrue, ObservedGeneration: 1}
	conditionTargets := []TargetCondition{
		{Name: "mysql-5", Projection: intent, Generation: 1, ObservedGeneration: 1, Condition: &failed5},
		{Name: "mysql-4", Projection: intent, Generation: 1, ObservedGeneration: 1, Condition: &completed},
		{Name: "mysql-3", Projection: intent, Generation: 1, ObservedGeneration: 1, Condition: &failed3},
	}
	status = ObserveTargetConditions(intent, "mysql", 7, 6, 3, conditionTargets)
	if status.Phase != appsv1.ReplicaRestoreFailed || status.RestoredReplicas != 1 || !strings.Contains(status.Message, "mysql-3") {
		t.Fatalf("unexpected deterministic Instance failure summary: %#v", status)
	}
}

func observedPVC(t *testing.T, intent *appsv1.ReplicaRestoreProjection, instanceName, pvcName, vct string, status corev1.ConditionStatus) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: pvcName, Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: vct},
	}}
	ApplyToPVC(pvc, intent, instanceName)
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type: corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore), Status: status,
	}}
	return pvc
}
