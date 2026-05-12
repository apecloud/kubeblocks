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

package cluster

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func applyClusterRestoreIntent(cluster *appsv1.Cluster, components []*appsv1.ClusterComponentSpec, shardings []*appsv1.ClusterSharding) error {
	completed := isClusterRestoreCompleted(cluster)
	if cluster.Spec.Restore == nil && !completed {
		return nil
	}
	for _, comp := range components {
		applyRestoreIntentToVCTs(cluster, comp.Name, comp.VolumeClaimTemplates, completed)
	}
	for _, sharding := range shardings {
		applyRestoreIntentToVCTs(cluster, sharding.Name, sharding.Template.VolumeClaimTemplates, completed)
		for i := range sharding.ShardTemplates {
			template := &sharding.ShardTemplates[i]
			applyRestoreIntentToVCTs(cluster, template.Name, template.VolumeClaimTemplates, completed)
		}
	}
	return nil
}

func isClusterRestoreCompleted(cluster *appsv1.Cluster) bool {
	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	return cond != nil && cond.Status == metav1.ConditionTrue
}

func applyRestoreIntentToVCTs(cluster *appsv1.Cluster, componentName string, vcts []appsv1.PersistentVolumeClaimTemplate, completed bool) {
	for i := range vcts {
		vct := &vcts[i]
		if completed {
			cleanupRestoreIntentFromVCT(vct)
			continue
		}
		injectRestoreIntentToVCT(cluster, componentName, vct)
	}
}

func injectRestoreIntentToVCT(cluster *appsv1.Cluster, componentName string, vct *appsv1.PersistentVolumeClaimTemplate) {
	restore := cluster.Spec.Restore
	if restore == nil {
		return
	}
	if vct.Annotations == nil {
		vct.Annotations = map[string]string{}
	}
	sourceNamespace := restore.Source.Namespace
	if sourceNamespace == "" {
		sourceNamespace = cluster.Namespace
	}
	vct.Annotations[constant.RestoreSourceAPIGroupAnnotationKey] = restore.Source.APIGroup
	vct.Annotations[constant.RestoreSourceKindAnnotationKey] = restore.Source.Kind
	vct.Annotations[constant.RestoreSourceNameAnnotationKey] = restore.Source.Name
	vct.Annotations[constant.RestoreSourceNamespaceAnnotationKey] = sourceNamespace
	vct.Annotations[constant.RestoreComponentAnnotationKey] = componentName
	vct.Annotations[constant.RestoreVolumeTemplateAnnotationKey] = vct.Name
	delete(vct.Annotations, constant.RestorePITRAnnotationKey)
	if restore.PITR != "" {
		vct.Annotations[constant.RestorePITRAnnotationKey] = restore.PITR
	}
	delete(vct.Annotations, constant.RestoreParametersAnnotationKey)
	if len(restore.Parameters) > 0 {
		if data, err := json.Marshal(restore.Parameters); err == nil {
			vct.Annotations[constant.RestoreParametersAnnotationKey] = string(data)
		}
	}
	apiGroup := restore.Source.APIGroup
	vct.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup: &apiGroup,
		Kind:     restore.Source.Kind,
		Name:     restore.Source.Name,
	}
}

func cleanupRestoreIntentFromVCT(vct *appsv1.PersistentVolumeClaimTemplate) {
	if !hasRestoreIntent(vct) {
		return
	}
	if vct.Annotations != nil {
		delete(vct.Annotations, constant.RestoreSourceAPIGroupAnnotationKey)
		delete(vct.Annotations, constant.RestoreSourceKindAnnotationKey)
		delete(vct.Annotations, constant.RestoreSourceNameAnnotationKey)
		delete(vct.Annotations, constant.RestoreSourceNamespaceAnnotationKey)
		delete(vct.Annotations, constant.RestorePITRAnnotationKey)
		delete(vct.Annotations, constant.RestoreParametersAnnotationKey)
		delete(vct.Annotations, constant.RestoreComponentAnnotationKey)
		delete(vct.Annotations, constant.RestoreVolumeTemplateAnnotationKey)
	}
	vct.Spec.DataSourceRef = nil
}

func hasRestoreIntent(vct *appsv1.PersistentVolumeClaimTemplate) bool {
	if vct.Annotations == nil {
		return false
	}
	for _, key := range []string{
		constant.RestoreSourceAPIGroupAnnotationKey,
		constant.RestoreSourceKindAnnotationKey,
		constant.RestoreSourceNameAnnotationKey,
		constant.RestoreSourceNamespaceAnnotationKey,
		constant.RestoreComponentAnnotationKey,
		constant.RestoreVolumeTemplateAnnotationKey,
	} {
		if _, ok := vct.Annotations[key]; ok {
			return true
		}
	}
	return false
}
