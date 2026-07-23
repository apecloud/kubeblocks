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

package systemaccount

import (
	"context"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

type RestoreOperationState string

const (
	RestoreOperationActive      RestoreOperationState = "Active"
	RestoreOperationSucceeded   RestoreOperationState = "TerminalSuccess"
	RestoreOperationFailed      RestoreOperationState = "TerminalFailure"
	RestoreOperationTerminating RestoreOperationState = "Terminating"
	RestoreOperationGone        RestoreOperationState = "Gone"
)

// ReadRestoreOperationState verifies that the live operation still matches the
// identity sealed into a restore request before Apps mutates account data.
func ReadRestoreOperationState(
	ctx context.Context,
	reader client.Reader,
	operation RestoreOperationIdentity,
) (RestoreOperationState, error) {
	if reader == nil {
		return "", fmt.Errorf("restore operation authority reader is not configured")
	}
	if operation.Root.APIVersion != appsv1.GroupVersion.String() ||
		operation.Root.Kind != appsv1.ClusterKind {
		return RestoreOperationGone, nil
	}
	cluster := &appsv1.Cluster{}
	if err := reader.Get(ctx, client.ObjectKey{
		Namespace: operation.Root.Namespace,
		Name:      operation.Root.Name,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return RestoreOperationGone, nil
		}
		return "", err
	}
	if cluster.UID != operation.Root.UID {
		return RestoreOperationGone, nil
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return RestoreOperationTerminating, nil
	}
	if operation.Profile == RestoreProfileLegacyPVCGroup {
		return readLegacyRestoreOperationState(ctx, reader, operation)
	}
	if operation.Profile != RestoreProfileInitialCluster || cluster.Spec.Restore == nil {
		return RestoreOperationGone, nil
	}
	namespace := cluster.Spec.Restore.Source.Namespace
	if namespace == "" {
		namespace = cluster.Namespace
	}
	current := RestoreOperationIdentity{
		Protocol: RestoreProtocolV2,
		Profile:  RestoreProfileInitialCluster,
		Root: ObjectIdentity{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       appsv1.ClusterKind,
			Namespace:  cluster.Namespace,
			Name:       cluster.Name,
			UID:        cluster.UID,
		},
		Source: SourceIdentity{
			APIGroup:  cluster.Spec.Restore.Source.APIGroup,
			Kind:      cluster.Spec.Restore.Source.Kind,
			Namespace: namespace,
			Name:      cluster.Spec.Restore.Source.Name,
		},
		PITR:       cluster.Spec.Restore.PITR,
		Parameters: maps.Clone(cluster.Spec.Restore.Parameters),
	}
	currentDigest, err := OperationDigest(current)
	if err != nil {
		return "", err
	}
	expectedDigest, err := OperationDigest(operation)
	if err != nil {
		return "", err
	}
	if currentDigest != expectedDigest {
		return RestoreOperationGone, nil
	}
	condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	if condition == nil || condition.ObservedGeneration != cluster.Generation ||
		condition.Status == metav1.ConditionUnknown {
		return RestoreOperationActive, nil
	}
	if condition.Status == metav1.ConditionTrue {
		return RestoreOperationSucceeded, nil
	}
	return RestoreOperationFailed, nil
}

// ReadRestoreTargetOwnerLive verifies the exact live Apps owner before a
// target credential write is planned.
func ReadRestoreTargetOwnerLive(
	ctx context.Context,
	reader client.Reader,
	identity ObjectIdentity,
) (bool, error) {
	if reader == nil {
		return false, fmt.Errorf("restore target owner authority reader is not configured")
	}
	if identity.APIVersion != appsv1.GroupVersion.String() {
		return false, nil
	}
	var owner client.Object
	switch identity.Kind {
	case appsv1.ComponentKind:
		owner = &appsv1.Component{}
	case appsv1.ClusterKind:
		owner = &appsv1.Cluster{}
	default:
		return false, nil
	}
	if err := reader.Get(ctx, client.ObjectKey{
		Namespace: identity.Namespace,
		Name:      identity.Name,
	}, owner); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return owner.GetUID() == identity.UID && owner.GetDeletionTimestamp().IsZero(), nil
}

func readLegacyRestoreOperationState(
	ctx context.Context,
	reader client.Reader,
	operation RestoreOperationIdentity,
) (RestoreOperationState, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := reader.List(ctx, pvcs,
		client.InNamespace(operation.Root.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: operation.Root.Name}); err != nil {
		return "", err
	}
	found := false
	allSucceeded := true
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !pvc.DeletionTimestamp.IsZero() ||
			!legacyPVCMatchesRestoreOperation(pvc, operation) {
			continue
		}
		authoritative, err := legacyPVCHasRestoreAuthority(ctx, reader, pvc, operation.Root)
		if err != nil {
			return "", err
		}
		if !authoritative {
			continue
		}
		found = true
		condition := findRestorePVCCondition(pvc, appsv1.ConditionTypeRestore)
		if condition == nil || condition.Status == corev1.ConditionUnknown {
			allSucceeded = false
			continue
		}
		if condition.Status == corev1.ConditionFalse {
			return RestoreOperationFailed, nil
		}
	}
	if !found {
		return RestoreOperationGone, nil
	}
	if allSucceeded {
		return RestoreOperationSucceeded, nil
	}
	return RestoreOperationActive, nil
}

func legacyPVCMatchesRestoreOperation(
	pvc *corev1.PersistentVolumeClaim,
	operation RestoreOperationIdentity,
) bool {
	if pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.APIGroup == nil ||
		*pvc.Spec.DataSourceRef.APIGroup != operation.Source.APIGroup ||
		pvc.Spec.DataSourceRef.Kind != operation.Source.Kind ||
		pvc.Spec.DataSourceRef.Name != operation.Source.Name ||
		pvc.Annotations[constant.RestorePITRAnnotationKey] != operation.PITR {
		return false
	}
	namespace := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		namespace = *pvc.Spec.DataSourceRef.Namespace
	}
	return namespace == operation.Source.Namespace
}

func legacyPVCHasRestoreAuthority(
	ctx context.Context,
	reader client.Reader,
	pvc *corev1.PersistentVolumeClaim,
	root ObjectIdentity,
) (bool, error) {
	ref := metav1.GetControllerOf(pvc)
	if ref == nil || ref.APIVersion != workloads.GroupVersion.String() {
		return false, nil
	}
	instanceSet := &workloads.InstanceSet{}
	switch ref.Kind {
	case workloads.InstanceSetKind:
		if err := reader.Get(ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      ref.Name,
		}, instanceSet); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if instanceSet.UID != ref.UID || !instanceSet.DeletionTimestamp.IsZero() {
			return false, nil
		}
	case "Instance":
		instance := &workloads.Instance{}
		if err := reader.Get(ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      ref.Name,
		}, instance); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if instance.UID != ref.UID || !instance.DeletionTimestamp.IsZero() {
			return false, nil
		}
		parent := metav1.GetControllerOf(instance)
		if parent == nil || parent.APIVersion != workloads.GroupVersion.String() ||
			parent.Kind != workloads.InstanceSetKind {
			return false, nil
		}
		if err := reader.Get(ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      parent.Name,
		}, instanceSet); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if instanceSet.UID != parent.UID || !instanceSet.DeletionTimestamp.IsZero() {
			return false, nil
		}
	default:
		return false, nil
	}
	componentRef := metav1.GetControllerOf(instanceSet)
	if componentRef == nil || componentRef.APIVersion != appsv1.GroupVersion.String() ||
		componentRef.Kind != appsv1.ComponentKind {
		return false, nil
	}
	component := &appsv1.Component{}
	if err := reader.Get(ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      componentRef.Name,
	}, component); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if component.UID != componentRef.UID || !component.DeletionTimestamp.IsZero() {
		return false, nil
	}
	clusterRef := metav1.GetControllerOf(component)
	return clusterRef != nil &&
		clusterRef.APIVersion == appsv1.GroupVersion.String() &&
		clusterRef.Kind == appsv1.ClusterKind &&
		clusterRef.Name == root.Name &&
		clusterRef.UID == root.UID, nil
}

func findRestorePVCCondition(
	pvc *corev1.PersistentVolumeClaim,
	conditionType corev1.PersistentVolumeClaimConditionType,
) *corev1.PersistentVolumeClaimCondition {
	for i := range pvc.Status.Conditions {
		if pvc.Status.Conditions[i].Type == conditionType {
			return &pvc.Status.Conditions[i]
		}
	}
	return nil
}
