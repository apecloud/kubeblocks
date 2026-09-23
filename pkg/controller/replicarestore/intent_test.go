/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd
*/

package replicarestore

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func testIntent() *appsv1.ReplicaRestoreProjection {
	group := "dataprotection.kubeblocks.io"
	return &appsv1.ReplicaRestoreProjection{
		StartOrdinal: 3,
		EndOrdinal:   5,
		SourceRef: corev1.TypedObjectReference{
			APIGroup: &group,
			Kind:     "Backup",
			Name:     "backup",
		},
		Annotations: map[string]string{"apps.kubeblocks.io/restore-source-name": "backup"},
		Fingerprint: "fp-1",
	}
}

func TestApplyToPVCTargetRange(t *testing.T) {
	intent := testIntent()
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:   "data-mysql-3",
		Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: "data"},
	}}
	ApplyToPVC(pvc, intent, "mysql-3")
	if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.Name != "backup" {
		t.Fatalf("expected target DataSourceRef, got %#v", pvc.Spec.DataSourceRef)
	}
	if pvc.Annotations[constant.RestorePurposeAnnotationKey] != constant.RestorePurposeReplica ||
		pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] != "data" ||
		pvc.Annotations[constant.ReplicaRestoreFingerprintAnnotationKey] != "fp-1" {
		t.Fatalf("unexpected target annotations: %#v", pvc.Annotations)
	}
}

func TestApplyToPVCClearsStaleRestoreOptions(t *testing.T) {
	intent := testIntent()
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:   "data-mysql-3",
		Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: "data"},
		Annotations: map[string]string{
			constant.RestorePITRAnnotationKey:            "old",
			constant.RestoreParametersAnnotationKey:      `{"old":"true"}`,
			constant.RestoreSourceNamespaceAnnotationKey: "old-ns",
		},
	}}
	ApplyToPVC(pvc, intent, "mysql-3")
	if _, ok := pvc.Annotations[constant.RestorePITRAnnotationKey]; ok {
		t.Fatal("stale PITR annotation was retained")
	}
	if _, ok := pvc.Annotations[constant.RestoreParametersAnnotationKey]; ok {
		t.Fatal("stale restore parameters were retained")
	}
	if _, ok := pvc.Annotations[constant.RestoreSourceNamespaceAnnotationKey]; ok {
		t.Fatal("stale source namespace was retained")
	}
}

func TestApplyToPVCLeavesNonTargetUnchanged(t *testing.T) {
	intent := testIntent()
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:        "data-mysql-2",
		Annotations: map[string]string{"keep": "me"},
		Labels:      map[string]string{constant.VolumeClaimTemplateNameLabelKey: "data"},
	}}
	ApplyToPVC(pvc, intent, "mysql-2")
	if pvc.Spec.DataSourceRef != nil || pvc.Annotations["keep"] != "me" || len(pvc.Annotations) != 1 {
		t.Fatalf("non-target PVC was changed: %#v", pvc)
	}
}

func TestValidateExistingPVCRejectsOrdinaryAndConflictingPVCs(t *testing.T) {
	intent := testIntent()
	desired := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-mysql-3"}}
	ApplyToPVC(desired, intent, "mysql-3")
	ordinary := desired.DeepCopy()
	ordinary.Spec.DataSourceRef = nil
	if err := ValidateExistingPVC(ordinary, desired, intent); err == nil {
		t.Fatal("expected ordinary PVC to be rejected")
	}
	conflict := desired.DeepCopy()
	conflict.Spec.DataSourceRef.Name = "other"
	if err := ValidateExistingPVC(conflict, desired, intent); err == nil {
		t.Fatal("expected conflicting source to be rejected")
	}
}

func TestValidateExistingPVCAcceptsMatchingTarget(t *testing.T) {
	intent := testIntent()
	desired := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name:   "data-mysql-3",
		Labels: map[string]string{constant.VolumeClaimTemplateNameLabelKey: "data"},
	}}
	ApplyToPVC(desired, intent, "mysql-3")
	if err := ValidateExistingPVC(desired.DeepCopy(), desired, intent); err != nil {
		t.Fatalf("expected matching PVC to be accepted: %v", err)
	}
}
