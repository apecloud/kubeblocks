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
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func applyClusterRestoreIntent(cluster *appsv1.Cluster, components []*appsv1.ClusterComponentSpec, shardings []*appsv1.ClusterSharding) error {
	return applyClusterRestoreIntentWithReader(context.Background(), nil, cluster, components, shardings)
}

func applyClusterRestoreIntentWithReader(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, components []*appsv1.ClusterComponentSpec, shardings []*appsv1.ClusterSharding) error {
	completed := isClusterRestoreCompleted(cluster)
	for _, comp := range components {
		if cluster.Spec.Restore != nil {
			applyRestoreIntentToComponent(cluster, comp.Name, comp.VolumeClaimTemplates, comp.Instances, completed)
		}
		if reader != nil {
			if err := validateReplicaRestoreIntent(ctx, reader, cluster, comp); err != nil {
				return err
			}
		}
	}
	for _, sharding := range shardings {
		if sharding.Template.ReplicaRestore != nil {
			return fmt.Errorf("sharding %q does not support replicaRestore", sharding.Name)
		}
		if cluster.Spec.Restore == nil {
			continue
		}
		applyRestoreIntentToComponent(cluster, sharding.Name, sharding.Template.VolumeClaimTemplates, sharding.Template.Instances, completed)
		for i := range sharding.ShardTemplates {
			template := &sharding.ShardTemplates[i]
			applyRestoreIntentToComponent(cluster, template.Name, template.VolumeClaimTemplates, template.Instances, completed)
		}
	}
	return nil
}

func validateReplicaRestoreIntent(ctx context.Context, reader client.Reader, cluster *appsv1.Cluster, comp *appsv1.ClusterComponentSpec) error {
	restore := comp.ReplicaRestore
	if restore == nil {
		return nil
	}
	if cluster.Spec.Restore != nil && !isClusterRestoreCompleted(cluster) {
		return fmt.Errorf("component %q replicaRestore requires initial Cluster restore to complete", comp.Name)
	}
	if len(comp.Instances) != 0 || len(comp.Ordinals.Ranges) != 0 || len(comp.Ordinals.Discrete) != 0 || comp.FlatInstanceOrdinal || len(comp.OfflineInstances) != 0 {
		return fmt.Errorf("component %q replicaRestore only supports default contiguous instances", comp.Name)
	}
	if comp.Replicas <= 0 {
		return fmt.Errorf("component %q replicaRestore requires replicas greater than zero", comp.Name)
	}
	if restore.Source.APIGroup != dptypes.DataprotectionAPIGroup || restore.Source.Kind != dptypes.BackupKind {
		return fmt.Errorf("component %q replicaRestore source must be a DataProtection Backup", comp.Name)
	}
	if reader == nil {
		return nil
	}
	component := &appsv1.Component{}
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: constant.GenerateClusterComponentName(cluster.Name, comp.Name)}
	if err := reader.Get(ctx, key, component); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("component %q replicaRestore requires an existing Component", comp.Name)
		}
		return err
	}
	if !component.DeletionTimestamp.IsZero() {
		return fmt.Errorf("component %q replicaRestore requires a non-deleting Component", comp.Name)
	}
	if len(component.Spec.Instances) != 0 || len(component.Spec.Ordinals.Ranges) != 0 || len(component.Spec.Ordinals.Discrete) != 0 || component.Spec.FlatInstanceOrdinal || len(component.Spec.OfflineInstances) != 0 {
		return fmt.Errorf("component %q replicaRestore requires a default contiguous live workload", comp.Name)
	}
	if len(component.Spec.VolumeClaimTemplates) == 0 || len(comp.VolumeClaimTemplates) == 0 {
		return fmt.Errorf("component %q replicaRestore requires at least one volume claim template", comp.Name)
	}
	if comp.Replicas < component.Spec.Replicas {
		return fmt.Errorf("component %q replicaRestore does not support scale-in; remove replicaRestore before reducing replicas", comp.Name)
	}
	return nil
}

func isClusterRestoreCompleted(cluster *appsv1.Cluster) bool {
	cond := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	return cond != nil && cond.Status == metav1.ConditionTrue
}

func applyRestoreIntentToComponent(cluster *appsv1.Cluster, componentName string, vcts []appsv1.PersistentVolumeClaimTemplate, instances []appsv1.InstanceTemplate, completed bool) {
	applyRestoreIntentToVCTs(cluster, componentName, vcts, completed)
	for i := range instances {
		applyRestoreIntentToVCTs(cluster, componentName, instances[i].VolumeClaimTemplates, completed)
	}
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
	vct.Annotations[constant.KBAppClusterUIDKey] = string(cluster.UID)
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
	var namespace *string
	if restore.Source.Namespace != "" && restore.Source.Namespace != cluster.Namespace {
		namespace = &restore.Source.Namespace
	}
	vct.Spec.DataSourceRef = &corev1.TypedObjectReference{
		APIGroup:  &apiGroup,
		Kind:      restore.Source.Kind,
		Name:      restore.Source.Name,
		Namespace: namespace,
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
		delete(vct.Annotations, constant.KBAppClusterUIDKey)
		delete(vct.Annotations, constant.RestorePITRAnnotationKey)
		delete(vct.Annotations, constant.RestoreParametersAnnotationKey)
		delete(vct.Annotations, constant.RestoreComponentAnnotationKey)
		delete(vct.Annotations, constant.RestoreVolumeTemplateAnnotationKey)
		if len(vct.Annotations) == 0 {
			vct.Annotations = nil
		}
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
		constant.KBAppClusterUIDKey,
		constant.RestoreComponentAnnotationKey,
		constant.RestoreVolumeTemplateAnnotationKey,
	} {
		if _, ok := vct.Annotations[key]; ok {
			return true
		}
	}
	return false
}
