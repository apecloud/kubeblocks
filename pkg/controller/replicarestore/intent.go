/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

package replicarestore

import (
	"encoding/json"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// ApplyToPVC adds the current owner restore intent to a newly built PVC.
// Existing PVCs are handled by the workload merge path, which preserves their
// source and restore annotations.
func ApplyToPVC(pvc *corev1.PersistentVolumeClaim, intent *appsv1.ClusterReplicaRestore, component string) {
	if pvc == nil || intent == nil {
		return
	}
	apiGroup := intent.Source.APIGroup
	namespace := intent.Source.Namespace
	if namespace == "" {
		namespace = pvc.Namespace
	}
	var namespaceRef *string
	if namespace != pvc.Namespace {
		namespaceRef = &namespace
	}
	pvc.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup:  &apiGroup,
		Kind:      intent.Source.Kind,
		Name:      intent.Source.Name,
		Namespace: namespaceRef,
	}
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	for key, value := range annotations(intent, component, pvc.Namespace) {
		pvc.Annotations[key] = value
	}
}

// ValidateExistingPVC only validates PVCs that already carry the replica
// marker. Ordinary existing PVCs remain owned by their original source and are
// never converted by a desired PVC merge.
func ValidateExistingPVC(existing, desired *corev1.PersistentVolumeClaim, intent *appsv1.ClusterReplicaRestore) error {
	if existing == nil || desired == nil || intent == nil || !IsReplicaPVC(existing) {
		return nil
	}
	// A completed PVC is an ordinary retained replica fact. It belongs to a
	// previous restore and must not block a later scale-out restore with a new
	// source. In-flight or failed PVCs remain protected by identity checks.
	for _, condition := range existing.Status.Conditions {
		if condition.Type == corev1.PersistentVolumeClaimConditionType(appsv1.ConditionTypeRestore) && condition.Status == corev1.ConditionTrue {
			return nil
		}
	}
	expected := desired.DeepCopy()
	ApplyToPVC(expected, intent, desired.Annotations[constant.RestoreComponentAnnotationKey])
	if !reflect.DeepEqual(existing.Spec.DataSourceRef, expected.Spec.DataSourceRef) {
		return fmt.Errorf("PVC %s does not match replica restore source", existing.Name)
	}
	for _, key := range []string{
		constant.RestorePurposeAnnotationKey,
		constant.RestoreSourceAPIGroupAnnotationKey,
		constant.RestoreSourceKindAnnotationKey,
		constant.RestoreSourceNameAnnotationKey,
		constant.RestoreSourceNamespaceAnnotationKey,
		constant.RestorePITRAnnotationKey,
		constant.RestoreParametersAnnotationKey,
	} {
		if existing.Annotations[key] != expected.Annotations[key] {
			return fmt.Errorf("PVC %s does not match replica restore annotation %s", existing.Name, key)
		}
	}
	return nil
}

func annotations(intent *appsv1.ClusterReplicaRestore, component, pvcNamespace string) map[string]string {
	sourceNamespace := intent.Source.Namespace
	if sourceNamespace == "" {
		sourceNamespace = pvcNamespace
	}
	result := map[string]string{
		constant.RestorePurposeAnnotationKey:         constant.RestorePurposeReplica,
		constant.RestoreSourceAPIGroupAnnotationKey:  intent.Source.APIGroup,
		constant.RestoreSourceKindAnnotationKey:      intent.Source.Kind,
		constant.RestoreSourceNameAnnotationKey:      intent.Source.Name,
		constant.RestoreSourceNamespaceAnnotationKey: sourceNamespace,
		constant.RestoreComponentAnnotationKey:       component,
	}
	if intent.PITR != "" {
		result[constant.RestorePITRAnnotationKey] = intent.PITR
	}
	parameters := map[string]string{}
	for key, value := range intent.Parameters {
		parameters[key] = value
	}
	if intent.SourceTargetName != "" {
		result[dptypes.SourceTargetNameAnnotationKey] = intent.SourceTargetName
		parameters[dptypes.SourceTargetNameAnnotationKey] = intent.SourceTargetName
	}
	if len(intent.Env) > 0 {
		data, _ := json.Marshal(intent.Env)
		parameters[dptypes.RestoreEnvParameterKey] = string(data)
	}
	if len(parameters) > 0 {
		data, _ := json.Marshal(parameters)
		result[constant.RestoreParametersAnnotationKey] = string(data)
	}
	return result
}

func IsReplicaPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc != nil && pvc.Annotations[constant.RestorePurposeAnnotationKey] == constant.RestorePurposeReplica
}
