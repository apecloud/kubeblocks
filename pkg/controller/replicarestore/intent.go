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
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// Applies reports whether a default contiguous ordinal belongs to the
// projected replica restore range.
func Applies(intent *appsv1.ReplicaRestoreProjection, instanceName string) bool {
	if intent == nil || intent.EndOrdinal <= intent.StartOrdinal {
		return false
	}
	idx := strings.LastIndexByte(instanceName, '-')
	if idx < 0 || idx == len(instanceName)-1 {
		return false
	}
	ordinal, err := strconv.ParseInt(instanceName[idx+1:], 10, 32)
	if err != nil {
		return false
	}
	return int32(ordinal) >= intent.StartOrdinal && int32(ordinal) < intent.EndOrdinal
}

// ApplyToPVC adds a replica restore source to a newly-built PVC. It does not
// mutate an existing PVC; callers must validate an existing object separately.
func ApplyToPVC(pvc *corev1.PersistentVolumeClaim, intent *appsv1.ReplicaRestoreProjection, instanceName string) {
	if pvc == nil || !Applies(intent, instanceName) {
		return
	}
	ref := intent.SourceRef.DeepCopy()
	pvc.Spec.DataSourceRef = ref
	if pvc.Annotations == nil {
		pvc.Annotations = map[string]string{}
	}
	for _, key := range []string{
		constant.ReplicaRestoreFingerprintAnnotationKey,
		constant.RestoreSourceAPIGroupAnnotationKey,
		constant.RestoreSourceKindAnnotationKey,
		constant.RestoreSourceNameAnnotationKey,
		constant.RestoreSourceNamespaceAnnotationKey,
		constant.RestorePITRAnnotationKey,
		constant.RestoreParametersAnnotationKey,
		constant.RestoreComponentAnnotationKey,
		constant.RestoreVolumeTemplateAnnotationKey,
		dptypes.SourceTargetNameAnnotationKey,
		dptypes.RestoreEnvParameterKey,
	} {
		delete(pvc.Annotations, key)
	}
	for key, value := range intent.Annotations {
		pvc.Annotations[key] = value
	}
	pvc.Annotations[constant.RestorePurposeAnnotationKey] = constant.RestorePurposeReplica
	if intent.Fingerprint != "" {
		pvc.Annotations[constant.ReplicaRestoreFingerprintAnnotationKey] = intent.Fingerprint
	}
	pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]
}

// ValidateExistingPVC prevents a normal PVC or a PVC from another restore
// source from being silently converted into a replica restore target.
func ValidateExistingPVC(existing, desired *corev1.PersistentVolumeClaim, intent *appsv1.ReplicaRestoreProjection) error {
	if existing == nil || desired == nil || intent == nil || !IsReplicaPVC(desired) {
		return nil
	}
	if existing.Spec.DataSourceRef == nil || !reflect.DeepEqual(existing.Spec.DataSourceRef, desired.Spec.DataSourceRef) {
		return fmt.Errorf("PVC %s/%s does not match replica restore source", existing.Namespace, existing.Name)
	}
	for _, key := range replicaRestoreMetadataKeys(intent) {
		if existing.Annotations[key] != desired.Annotations[key] {
			return fmt.Errorf("PVC %s/%s does not match replica restore annotation %s", existing.Namespace, existing.Name, key)
		}
	}
	if existing.Labels[constant.VolumeClaimTemplateNameLabelKey] != desired.Labels[constant.VolumeClaimTemplateNameLabelKey] {
		return fmt.Errorf("PVC %s/%s does not match replica restore volume claim template", existing.Namespace, existing.Name)
	}
	return nil
}

func replicaRestoreMetadataKeys(intent *appsv1.ReplicaRestoreProjection) []string {
	keys := []string{
		constant.RestorePurposeAnnotationKey,
		constant.ReplicaRestoreFingerprintAnnotationKey,
		constant.RestoreSourceAPIGroupAnnotationKey,
		constant.RestoreSourceKindAnnotationKey,
		constant.RestoreSourceNameAnnotationKey,
		constant.RestoreSourceNamespaceAnnotationKey,
		constant.RestorePITRAnnotationKey,
		constant.RestoreParametersAnnotationKey,
		constant.RestoreComponentAnnotationKey,
		constant.RestoreVolumeTemplateAnnotationKey,
		dptypes.SourceTargetNameAnnotationKey,
		dptypes.RestoreEnvParameterKey,
	}
	for key := range intent.Annotations {
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys
}

func IsReplicaPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc != nil && pvc.Annotations[constant.RestorePurposeAnnotationKey] == constant.RestorePurposeReplica
}
