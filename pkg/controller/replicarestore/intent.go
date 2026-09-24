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
	"slices"

	corev1 "k8s.io/api/core/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

// ApplyToPVC adds the current owner restore intent to a newly built PVC.
// Existing PVCs are handled by the workload merge path, which preserves their
// source and restore annotations.
func ApplyToPVC(pvc *corev1.PersistentVolumeClaim, intent *appsv1.ClusterReplicaRestore, component, clusterUID string) {
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

// MergePVCAnnotations preserves an existing replica PVC's restore source, even
// after its owner changes or removes replicaRestore.
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
	dptypes.SourceTargetNameAnnotationKey,
	dptypes.RestoreEnvParameterKey,
}
