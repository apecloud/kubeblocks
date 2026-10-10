/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

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

package replicarestore

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

// ApplyToPVC adds the current owner restore intent to a newly built PVC.
// Existing PVCs are handled by the workload merge path, which preserves their
// source and restore annotations.
func ApplyToPVC(pvc *corev1.PersistentVolumeClaim, intent *appsv1.ClusterRestore, component, clusterUID string) error {
	if pvc == nil || intent == nil {
		return nil
	}
	if pvc.Spec.DataSource != nil || pvc.Spec.DataSourceRef != nil {
		return fmt.Errorf("PVC %q: replicaRestore cannot be combined with volume claim template dataSource or dataSourceRef", pvc.Name)
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
	for _, key := range metadataKeys {
		delete(pvc.Annotations, key)
	}
	if clusterUID != "" {
		pvc.Annotations[constant.KBAppClusterUIDKey] = clusterUID
	}
	pvc.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = pvc.Labels[constant.VolumeClaimTemplateNameLabelKey]
	for key, value := range annotations(intent, component, pvc.Namespace) {
		pvc.Annotations[key] = value
	}
	return nil
}

func annotations(intent *appsv1.ClusterRestore, component, pvcNamespace string) map[string]string {
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
	if len(parameters) > 0 {
		data, _ := json.Marshal(parameters)
		result[constant.RestoreParametersAnnotationKey] = string(data)
	}
	return result
}

func IsReplicaPVC(pvc *corev1.PersistentVolumeClaim) bool {
	return pvc != nil && pvc.Annotations[constant.RestorePurposeAnnotationKey] == constant.RestorePurposeReplica
}

// ValidatePVCReuse rejects existing volumes with a different initialization input.
// Call it when allocating a restore replica, before merging any PVC metadata.
func ValidatePVCReuse(existing, desired *corev1.PersistentVolumeClaim) error {
	if !IsReplicaPVC(desired) {
		return nil
	}
	conflict := fmt.Errorf("PVC %s/%s: existing initialization input is incompatible with replicaRestore", existing.Namespace, existing.Name)
	if !IsReplicaPVC(existing) || !reflect.DeepEqual(existing.Spec.DataSourceRef, desired.Spec.DataSourceRef) {
		return conflict
	}
	for _, key := range metadataKeys {
		if existing.Annotations[key] != desired.Annotations[key] {
			return conflict
		}
	}
	return nil
}

// MergePVCAnnotations preserves existing PVC sources and restore metadata
// during workload updates.
func MergePVCAnnotations(existing, desired *corev1.PersistentVolumeClaim) {
	for key, value := range desired.Annotations {
		if IsReplicaPVC(existing) || IsReplicaPVC(desired) {
			if slices.Contains(metadataKeys, key) {
				continue
			}
		}
		if existing.Annotations == nil {
			existing.Annotations = map[string]string{}
		}
		existing.Annotations[key] = value
	}
}

var metadataKeys = []string{
	constant.KBAppClusterUIDKey,
	constant.RestorePurposeAnnotationKey,
	constant.RestoreSourceAPIGroupAnnotationKey,
	constant.RestoreSourceKindAnnotationKey,
	constant.RestoreSourceNameAnnotationKey,
	constant.RestoreSourceNamespaceAnnotationKey,
	constant.RestorePITRAnnotationKey,
	constant.RestoreParametersAnnotationKey,
	constant.RestoreComponentAnnotationKey,
	constant.RestoreVolumeTemplateAnnotationKey,
}
