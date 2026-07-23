/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package dataprotection

import (
	"errors"
	"fmt"
	"maps"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

type systemAccountRestoreAuthority struct {
	root      *appsv1.Cluster
	component *appsv1.Component
	operation systemaccount.RestoreOperationIdentity
}

type systemAccountRestoreContractError struct {
	reason string
	cause  error
}

func (e *systemAccountRestoreContractError) Error() string {
	return fmt.Sprintf("%s: %v", e.reason, e.cause)
}

func (e *systemAccountRestoreContractError) Unwrap() error {
	return e.cause
}

func newSystemAccountRestoreContractError(reason string, err error) error {
	if err == nil {
		err = errors.New(reason)
	}
	return &systemAccountRestoreContractError{reason: reason, cause: err}
}

func (r *VolumePopulatorReconciler) buildSystemAccountCredentialIntent(
	reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup,
	scope systemAccountSecretScope,
	clusterName, ownerName, accountName string,
	password []byte,
) (systemaccount.CredentialIntent, error) {
	authority, err := r.resolveSystemAccountRestoreAuthority(reqCtx, pvc, backup, clusterName)
	if err != nil {
		var contractErr *systemAccountRestoreContractError
		if !apierrors.IsNotFound(err) && !errors.As(err, &contractErr) {
			err = newSystemAccountRestoreContractError(
				systemaccount.UnauthorizedRestoreProducerReason, err)
		}
		return systemaccount.CredentialIntent{}, err
	}
	targetOwner := objectIdentity(authority.component)
	shardingName := ""
	switch scope {
	case systemAccountSecretScopeComponent:
		if authority.component.Labels[constant.KBAppComponentLabelKey] != ownerName &&
			authority.component.Name != constant.GenerateClusterComponentName(clusterName, ownerName) {
			return systemaccount.CredentialIntent{}, newSystemAccountRestoreContractError(
				systemaccount.UnauthorizedRestoreProducerReason,
				fmt.Errorf("PVC %s/%s component identity %q does not match live Component %s/%s",
					pvc.Namespace, pvc.Name, ownerName, authority.component.Namespace, authority.component.Name))
		}
	case systemAccountSecretScopeSharding:
		if pvc.Labels[constant.KBAppShardingNameLabelKey] != ownerName {
			return systemaccount.CredentialIntent{}, newSystemAccountRestoreContractError(
				systemaccount.UnauthorizedRestoreProducerReason,
				fmt.Errorf("PVC %s/%s sharding identity %q does not match %q",
					pvc.Namespace, pvc.Name, ownerName, pvc.Labels[constant.KBAppShardingNameLabelKey]))
		}
		targetOwner = objectIdentity(authority.root)
		shardingName = ownerName
	default:
		return systemaccount.CredentialIntent{}, newSystemAccountRestoreContractError(
			systemaccount.TargetSemanticUnavailableReason,
			fmt.Errorf("unsupported system account scope %q", scope))
	}
	rootIdentity := objectIdentity(authority.root)
	return systemaccount.CredentialIntent{
		Operation: authority.operation,
		Target: systemaccount.LogicalTargetIdentity{
			Protocol:     systemaccount.LogicalTargetProtocolV1,
			Namespace:    authority.root.Namespace,
			Root:         rootIdentity,
			Owner:        targetOwner,
			Scope:        string(scope),
			ShardingName: shardingName,
			Account:      accountName,
		},
		ResolvedSource: objectIdentity(backup),
		Credentials: map[string][]byte{
			constant.AccountNameForSecret:   []byte(accountName),
			constant.AccountPasswdForSecret: append([]byte(nil), password...),
		},
	}, nil
}

func (r *VolumePopulatorReconciler) resolveSystemAccountRestoreAuthority(
	reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup,
	clusterName string,
) (*systemAccountRestoreAuthority, error) {
	if pvc == nil || backup == nil {
		return nil, fmt.Errorf("PVC and Backup are required")
	}
	if pvc.Namespace == "" || pvc.UID == "" {
		return nil, fmt.Errorf("PVC identity must include namespace and UID")
	}
	if !pvc.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("PVC %s/%s is terminating", pvc.Namespace, pvc.Name)
	}
	workload, err := r.resolveRestorePVCWorkload(reqCtx, pvc)
	if err != nil {
		return nil, err
	}
	componentRef := metav1.GetControllerOf(workload)
	if componentRef == nil || componentRef.APIVersion != appsv1.GroupVersion.String() ||
		componentRef.Kind != "Component" {
		return nil, fmt.Errorf("PVC %s/%s workload %s/%s has no Component controller owner",
			pvc.Namespace, pvc.Name, workload.GetNamespace(), workload.GetName())
	}
	component := &appsv1.Component{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      componentRef.Name,
	}, component); err != nil {
		return nil, err
	}
	if component.UID == "" || component.UID != componentRef.UID {
		return nil, fmt.Errorf("PVC %s/%s Component owner UID mismatch", pvc.Namespace, pvc.Name)
	}
	if !component.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("PVC %s/%s Component owner is terminating", pvc.Namespace, pvc.Name)
	}
	clusterRef := metav1.GetControllerOf(component)
	if clusterRef == nil || clusterRef.APIVersion != appsv1.GroupVersion.String() ||
		clusterRef.Kind != "Cluster" || clusterRef.Name != clusterName {
		return nil, fmt.Errorf("component %s/%s has no matching Cluster %q controller owner",
			component.Namespace, component.Name, clusterName)
	}
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      clusterRef.Name,
	}, cluster); err != nil {
		return nil, err
	}
	if cluster.UID == "" || cluster.UID != clusterRef.UID {
		return nil, fmt.Errorf("component %s/%s Cluster owner UID mismatch", component.Namespace, component.Name)
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("component %s/%s Cluster owner is terminating", component.Namespace, component.Name)
	}
	if workload.GetLabels()[constant.AppInstanceLabelKey] != cluster.Name ||
		pvc.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return nil, fmt.Errorf("PVC %s/%s workload authority does not match Cluster %s/%s",
			pvc.Namespace, pvc.Name, cluster.Namespace, cluster.Name)
	}
	source := systemaccount.SourceIdentity{
		APIGroup:  dptypes.DataprotectionAPIGroup,
		Kind:      dptypes.BackupKind,
		Namespace: backup.Namespace,
		Name:      backup.Name,
	}
	operation := systemaccount.RestoreOperationIdentity{
		Protocol: systemaccount.RestoreProtocolV2,
		Root:     objectIdentity(cluster),
		Source:   source,
	}
	switch {
	case cluster.Spec.Restore != nil:
		declared := cluster.Spec.Restore.Source
		namespace := declared.Namespace
		if namespace == "" {
			namespace = cluster.Namespace
		}
		if declared.APIGroup != source.APIGroup || declared.Kind != source.Kind ||
			namespace != source.Namespace || declared.Name != source.Name {
			return nil, newSystemAccountRestoreContractError(
				systemaccount.RestoreIntentMismatchReason,
				fmt.Errorf("cluster %s/%s restore source does not match Backup %s/%s",
					cluster.Namespace, cluster.Name, backup.Namespace, backup.Name))
		}
		operation.Profile = systemaccount.RestoreProfileInitialCluster
		operation.Source.Namespace = namespace
		operation.PITR = cluster.Spec.Restore.PITR
		operation.Parameters = maps.Clone(cluster.Spec.Restore.Parameters)
	case backup.Namespace != pvc.Namespace:
		return nil, newSystemAccountRestoreContractError(
			systemaccount.UnsupportedRestoreSourceReason,
			fmt.Errorf("PVC %s/%s legacy restore profile does not allow cross-namespace Backup %s/%s",
				pvc.Namespace, pvc.Name, backup.Namespace, backup.Name))
	default:
		operation.Profile = systemaccount.RestoreProfileLegacyPVCGroup
		operation.PITR = pvc.Annotations[constant.RestorePITRAnnotationKey]
	}
	return &systemAccountRestoreAuthority{
		root:      cluster,
		component: component,
		operation: operation,
	}, nil
}

func (r *VolumePopulatorReconciler) resolveRestorePVCWorkload(
	reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
) (client.Object, error) {
	ownerRef := metav1.GetControllerOf(pvc)
	if ownerRef == nil || ownerRef.APIVersion != workloads.GroupVersion.String() {
		return nil, fmt.Errorf("PVC %s/%s has no supported workload controller owner", pvc.Namespace, pvc.Name)
	}
	var workload client.Object
	switch ownerRef.Kind {
	case workloads.InstanceSetKind:
		workload = &workloads.InstanceSet{}
	case "Instance":
		workload = &workloads.Instance{}
	default:
		return nil, fmt.Errorf("PVC %s/%s has unsupported workload owner kind %q",
			pvc.Namespace, pvc.Name, ownerRef.Kind)
	}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      ownerRef.Name,
	}, workload); err != nil {
		return nil, err
	}
	if workload.GetUID() == "" || workload.GetUID() != ownerRef.UID {
		return nil, fmt.Errorf("PVC %s/%s workload owner UID mismatch", pvc.Namespace, pvc.Name)
	}
	if !workload.GetDeletionTimestamp().IsZero() {
		return nil, fmt.Errorf("PVC %s/%s workload owner is terminating", pvc.Namespace, pvc.Name)
	}
	return workload, nil
}

func objectIdentity(object client.Object) systemaccount.ObjectIdentity {
	gvk := object.GetObjectKind().GroupVersionKind()
	if gvk.Empty() {
		switch object.(type) {
		case *appsv1.Cluster:
			gvk = appsv1.GroupVersion.WithKind("Cluster")
		case *appsv1.Component:
			gvk = appsv1.GroupVersion.WithKind("Component")
		case *dpv1alpha1.Backup:
			gvk = dpv1alpha1.GroupVersion.WithKind("Backup")
		}
	}
	return systemaccount.ObjectIdentity{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Namespace:  object.GetNamespace(),
		Name:       object.GetName(),
		UID:        object.GetUID(),
	}
}

func (r *VolumePopulatorReconciler) projectSystemAccountConflictIdentity(
	reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	envelope systemaccount.ConflictEnvelope,
) error {
	if pvc == nil || pvc.Namespace == "" || pvc.Name == "" || pvc.UID == "" {
		return fmt.Errorf("PVC identity must include namespace, name, and UID")
	}
	expected, err := systemAccountConflictExpectedIdentity(envelope)
	if err != nil {
		return err
	}

	current := &corev1.PersistentVolumeClaim{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), current); err != nil {
		return err
	}
	if current.UID != pvc.UID {
		return fmt.Errorf("PVC %s was replaced: expected UID %s, got %s",
			client.ObjectKeyFromObject(pvc), pvc.UID, current.UID)
	}
	if systemAccountConflictIdentityExact(current, expected) {
		*pvc = *current
		return nil
	}

	updated := current.DeepCopy()
	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	for key, value := range expected {
		updated.Annotations[key] = value
	}
	updateErr := r.Client.Update(reqCtx.Ctx, updated)
	if updateErr == nil {
		*pvc = *updated
		return nil
	}

	// An update response can be lost after the apiserver committed it. Accept
	// only a fresh read of the same PVC UID with all three receipt identities.
	fresh := &corev1.PersistentVolumeClaim{}
	if getErr := r.Client.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), fresh); getErr == nil &&
		fresh.UID == pvc.UID && systemAccountConflictIdentityExact(fresh, expected) {
		*pvc = *fresh
		return nil
	}
	return updateErr
}

func systemAccountConflictExpectedIdentity(
	envelope systemaccount.ConflictEnvelope,
) (map[string]string, error) {
	operationDigest, err := systemaccount.OperationDigest(envelope.LoserOperation)
	if err != nil {
		return nil, err
	}
	expected := map[string]string{
		systemaccount.RestoreOperationDigestAnnotationKey: operationDigest,
		systemaccount.BlockingRequestUIDAnnotationKey:     string(envelope.BlockingRequest.UID),
		systemaccount.WinnerOperationDigestAnnotationKey:  envelope.BlockingRequest.WinnerOperationDigest,
	}
	for key, value := range expected {
		if value == "" {
			return nil, fmt.Errorf("conflict projection annotation %q must not be empty", key)
		}
	}
	return expected, nil
}

func systemAccountConflictIdentityExact(
	pvc *corev1.PersistentVolumeClaim,
	expected map[string]string,
) bool {
	if pvc == nil || pvc.Annotations == nil {
		return false
	}
	for key, value := range expected {
		if value == "" || pvc.Annotations[key] != value {
			return false
		}
	}
	return true
}

func systemAccountConflictIdentityPresent(pvc *corev1.PersistentVolumeClaim) bool {
	if pvc == nil || pvc.Annotations == nil {
		return false
	}
	for _, key := range []string{
		systemaccount.RestoreOperationDigestAnnotationKey,
		systemaccount.BlockingRequestUIDAnnotationKey,
		systemaccount.WinnerOperationDigestAnnotationKey,
	} {
		if pvc.Annotations[key] == "" {
			return false
		}
	}
	return true
}

func (r *VolumePopulatorReconciler) systemAccountRestoreOperationState(
	reqCtx intctrlutil.RequestCtx,
	operation systemaccount.RestoreOperationIdentity,
) (systemAccountRestoreOperationState, error) {
	cluster := &appsv1.Cluster{}
	if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: operation.Root.Namespace,
		Name:      operation.Root.Name,
	}, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return systemAccountRestoreOperationGone, nil
		}
		return "", err
	}
	if cluster.UID != operation.Root.UID {
		return systemAccountRestoreOperationGone, nil
	}
	if !cluster.DeletionTimestamp.IsZero() {
		return systemAccountRestoreOperationTerminating, nil
	}
	if operation.Profile == systemaccount.RestoreProfileInitialCluster {
		if cluster.Spec.Restore == nil {
			return systemAccountRestoreOperationGone, nil
		}
		current := systemaccount.RestoreOperationIdentity{
			Protocol: systemaccount.RestoreProtocolV2,
			Profile:  systemaccount.RestoreProfileInitialCluster,
			Root:     objectIdentity(cluster),
			Source: systemaccount.SourceIdentity{
				APIGroup:  cluster.Spec.Restore.Source.APIGroup,
				Kind:      cluster.Spec.Restore.Source.Kind,
				Namespace: cluster.Spec.Restore.Source.Namespace,
				Name:      cluster.Spec.Restore.Source.Name,
			},
			PITR:       cluster.Spec.Restore.PITR,
			Parameters: maps.Clone(cluster.Spec.Restore.Parameters),
		}
		if current.Source.Namespace == "" {
			current.Source.Namespace = cluster.Namespace
		}
		currentDigest, err := systemaccount.OperationDigest(current)
		if err != nil {
			return "", err
		}
		expectedDigest, err := systemaccount.OperationDigest(operation)
		if err != nil {
			return "", err
		}
		if currentDigest != expectedDigest {
			return systemAccountRestoreOperationGone, nil
		}
		condition := findClusterCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
		if condition == nil || condition.ObservedGeneration != cluster.Generation ||
			condition.Status == metav1.ConditionUnknown {
			return systemAccountRestoreOperationActive, nil
		}
		if condition.Status == metav1.ConditionTrue {
			return systemAccountRestoreOperationSucceeded, nil
		}
		return systemAccountRestoreOperationFailed, nil
	}
	return r.legacySystemAccountRestoreOperationState(reqCtx, operation)
}

type systemAccountRestoreOperationState string

const (
	systemAccountRestoreOperationActive      systemAccountRestoreOperationState = "Active"
	systemAccountRestoreOperationSucceeded   systemAccountRestoreOperationState = "Succeeded"
	systemAccountRestoreOperationFailed      systemAccountRestoreOperationState = "Failed"
	systemAccountRestoreOperationTerminating systemAccountRestoreOperationState = "Terminating"
	systemAccountRestoreOperationGone        systemAccountRestoreOperationState = "Gone"
)

func (r *VolumePopulatorReconciler) legacySystemAccountRestoreOperationState(
	reqCtx intctrlutil.RequestCtx,
	operation systemaccount.RestoreOperationIdentity,
) (systemAccountRestoreOperationState, error) {
	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := r.Client.List(reqCtx.Ctx, pvcs,
		client.InNamespace(operation.Root.Namespace),
		client.MatchingLabels{constant.AppInstanceLabelKey: operation.Root.Name}); err != nil {
		return "", err
	}
	found := false
	allSucceeded := true
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		if !pvcReferencesOperationSource(pvc, operation) {
			continue
		}
		namespace, err := backupNamespaceFromPVC(pvc)
		if err != nil || namespace != operation.Source.Namespace {
			continue
		}
		workload, err := r.resolveRestorePVCWorkload(reqCtx, pvc)
		if err != nil {
			continue
		}
		componentRef := metav1.GetControllerOf(workload)
		if componentRef == nil || componentRef.Kind != "Component" {
			continue
		}
		component := &appsv1.Component{}
		if err := r.Client.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      componentRef.Name,
		}, component); err != nil || component.UID != componentRef.UID {
			continue
		}
		clusterRef := metav1.GetControllerOf(component)
		if clusterRef == nil || clusterRef.UID != operation.Root.UID {
			continue
		}
		found = true
		condition := findPVCConditionByType(pvc, appsv1.ConditionTypeRestore)
		if condition == nil || condition.Status == corev1.ConditionUnknown {
			allSucceeded = false
			continue
		}
		if condition.Status == corev1.ConditionFalse {
			return systemAccountRestoreOperationFailed, nil
		}
	}
	if !found {
		return systemAccountRestoreOperationGone, nil
	}
	if allSucceeded {
		return systemAccountRestoreOperationSucceeded, nil
	}
	return systemAccountRestoreOperationActive, nil
}

func findClusterCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
