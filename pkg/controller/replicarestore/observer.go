/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

SPDX-License-Identifier: AGPL-3.0-or-later
*/

package replicarestore

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// Target describes the PVCs that must finish restoring before an instance is
// counted as restored.
type Target struct {
	InstanceName string
	PVCs         []PVCIdentity
}

// PVCIdentity identifies one effective volume claim template assignment.
type PVCIdentity struct {
	Name                string
	VolumeClaimTemplate string
}

// TargetCondition is the Instance API observation used by an InstanceSet.
type TargetCondition struct {
	Name               string
	Projection         *appsv1.ReplicaRestoreProjection
	Generation         int64
	ObservedGeneration int64
	Condition          *metav1.Condition
}

// ObservePVCs returns restore progress grouped by target instance. An instance
// is restored only after every effective VCT PVC has completed.
func ObservePVCs(intent *appsv1.ReplicaRestoreProjection, component string, generation int64,
	targetReplicas, expectedTargets int32, targets []Target, pvcs []*corev1.PersistentVolumeClaim) *appsv1.ReplicaRestoreStatus {
	if intent == nil || intent.EndOrdinal <= intent.StartOrdinal {
		return nil
	}
	status := newStatus(component, generation, targetReplicas)
	pvcByName := make(map[string]*corev1.PersistentVolumeClaim, len(pvcs))
	for _, pvc := range pvcs {
		pvcByName[pvc.Name] = pvc
	}
	waiting := make([]string, 0)
	failures := make([]string, 0)
	for _, target := range targets {
		complete := len(target.PVCs) > 0
		if !complete {
			waiting = append(waiting, fmt.Sprintf("Instance %s volume claim templates", target.InstanceName))
		}
		for _, expected := range target.PVCs {
			pvc := pvcByName[expected.Name]
			if pvc == nil {
				complete = false
				waiting = append(waiting, expected.Name)
				continue
			}
			if reason := validateObservedPVC(pvc, expected, intent); reason != "" {
				complete = false
				failures = append(failures, reason)
				continue
			}
			condition := restoreCondition(pvc)
			if condition == nil || condition.Status == corev1.ConditionUnknown {
				complete = false
				waiting = append(waiting, expected.Name)
				continue
			}
			if condition.Status == corev1.ConditionFalse {
				complete = false
				failures = append(failures, fmt.Sprintf("PVC %s restore failed: %s", pvc.Name, condition.Message))
			}
		}
		if complete {
			status.RestoredReplicas++
		}
	}
	if int32(len(targets)) != expectedTargets {
		waiting = append(waiting, fmt.Sprintf("expected %d target instances, observed %d", expectedTargets, len(targets)))
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		status.Phase = appsv1.ReplicaRestoreFailed
		status.Message = failures[0]
		return status
	}
	if len(waiting) == 0 && status.RestoredReplicas == expectedTargets && expectedTargets > 0 {
		status.Phase = appsv1.ReplicaRestoreCompleted
		status.Message = "all replica restore instances completed"
		return status
	}
	if len(waiting) > 0 {
		sort.Strings(waiting)
		status.Phase = appsv1.ReplicaRestorePending
		status.Message = fmt.Sprintf("waiting for replica restore PVCs: %s", strings.Join(waiting, ","))
	}
	return status
}

// ObserveTargetConditions aggregates current, projection-matched Instance
// conditions into the same summary used by the PVC observer.
func ObserveTargetConditions(intent *appsv1.ReplicaRestoreProjection, component string, generation int64,
	targetReplicas, expectedTargets int32, targets []TargetCondition) *appsv1.ReplicaRestoreStatus {
	if intent == nil || intent.EndOrdinal <= intent.StartOrdinal {
		return nil
	}
	status := newStatus(component, generation, targetReplicas)
	waiting := make([]string, 0)
	failures := make([]string, 0)
	for _, target := range targets {
		if !reflect.DeepEqual(target.Projection, intent) {
			waiting = append(waiting, fmt.Sprintf("Instance %s replica restore projection", target.Name))
			continue
		}
		condition := target.Condition
		if target.ObservedGeneration != target.Generation || condition == nil || condition.ObservedGeneration != target.Generation || condition.Status == metav1.ConditionUnknown {
			waiting = append(waiting, target.Name)
			continue
		}
		if condition.Status == metav1.ConditionFalse {
			failures = append(failures, fmt.Sprintf("Instance %s restore failed: %s", target.Name, condition.Message))
			continue
		}
		status.RestoredReplicas++
	}
	if int32(len(targets)) != expectedTargets {
		waiting = append(waiting, fmt.Sprintf("expected %d target instances, observed %d", expectedTargets, len(targets)))
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		status.Phase = appsv1.ReplicaRestoreFailed
		status.Message = failures[0]
		return status
	}
	if len(waiting) == 0 && status.RestoredReplicas == expectedTargets && expectedTargets > 0 {
		status.Phase = appsv1.ReplicaRestoreCompleted
		status.Message = "all replica restore instances completed"
		return status
	}
	if len(waiting) > 0 {
		sort.Strings(waiting)
		status.Phase = appsv1.ReplicaRestorePending
		status.Message = fmt.Sprintf("waiting for replica restore Instances: %s", strings.Join(waiting, ","))
	}
	return status
}

// ConditionFromStatus preserves the existing workload condition contract while
// ReplicaRestoreStatus carries counts between controller layers.
func ConditionFromStatus(status *appsv1.ReplicaRestoreStatus, conditionType string) *metav1.Condition {
	if status == nil {
		return nil
	}
	condition := &metav1.Condition{
		Type: conditionType, ObservedGeneration: status.ObservedGeneration,
		Status: metav1.ConditionUnknown, Reason: "RestoreRunning", Message: status.Message,
	}
	switch status.Phase {
	case appsv1.ReplicaRestoreCompleted:
		condition.Status = metav1.ConditionTrue
		condition.Reason = "RestoreCompleted"
	case appsv1.ReplicaRestoreFailed:
		condition.Status = metav1.ConditionFalse
		condition.Reason = "RestoreFailed"
	}
	return condition
}

func newStatus(component string, generation int64, targetReplicas int32) *appsv1.ReplicaRestoreStatus {
	return &appsv1.ReplicaRestoreStatus{
		Component: component, Phase: appsv1.ReplicaRestoreRunning,
		TargetReplicas: targetReplicas, ObservedGeneration: generation,
	}
}

func validateObservedPVC(pvc *corev1.PersistentVolumeClaim, expected PVCIdentity, intent *appsv1.ReplicaRestoreProjection) string {
	if !IsReplicaPVC(pvc) {
		return fmt.Sprintf("PVC %s is not marked as a replica restore target", pvc.Name)
	}
	if !reflect.DeepEqual(pvc.Spec.DataSourceRef, &intent.SourceRef) {
		return fmt.Sprintf("PVC %s does not match replica restore source", pvc.Name)
	}
	if intent.Fingerprint != "" && pvc.Annotations[constant.ReplicaRestoreFingerprintAnnotationKey] != intent.Fingerprint {
		return fmt.Sprintf("PVC %s does not match replica restore fingerprint", pvc.Name)
	}
	if pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] != expected.VolumeClaimTemplate ||
		pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] != expected.VolumeClaimTemplate {
		return fmt.Sprintf("PVC %s does not match volume claim template %s", pvc.Name, expected.VolumeClaimTemplate)
	}
	for key, value := range intent.Annotations {
		if pvc.Annotations[key] != value {
			return fmt.Sprintf("PVC %s does not match replica restore annotation %s", pvc.Name, key)
		}
	}
	return ""
}

func restoreCondition(pvc *corev1.PersistentVolumeClaim) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if string(pvc.Status.Conditions[i].Type) == appsv1.ConditionTypeRestore {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}
