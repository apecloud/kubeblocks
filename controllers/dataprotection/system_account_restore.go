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
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/sharding"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

type systemAccountRestoreAuthority struct {
	pvc       *corev1.PersistentVolumeClaim
	root      *appsv1.Cluster
	component *appsv1.Component
	workload  *workloads.InstanceSet
	source    *dpv1alpha1.Backup
	operation systemaccount.RestoreOperationIdentity
}

type systemAccountRestoreAuthorityMode uint8

const (
	systemAccountRestoreAuthorityForRequest systemAccountRestoreAuthorityMode = iota
	systemAccountRestoreAuthorityForConflictProjection
)

type systemAccountRestoreContractError struct {
	reason string
	cause  error
}

type systemAccountRestoreAuthorityReadError struct {
	cause error
}

func (e *systemAccountRestoreAuthorityReadError) Error() string {
	return e.cause.Error()
}

func (e *systemAccountRestoreAuthorityReadError) Unwrap() error {
	return e.cause
}

func systemAccountAuthorityReadError(err error) error {
	if err == nil || apierrors.IsNotFound(err) {
		return err
	}
	return &systemAccountRestoreAuthorityReadError{cause: err}
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
		var readErr *systemAccountRestoreAuthorityReadError
		if errors.As(err, &readErr) {
			return systemaccount.CredentialIntent{}, err
		}
		var contractErr *systemAccountRestoreContractError
		if !errors.As(err, &contractErr) {
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
		if err := r.validateSystemAccountShardingAuthority(
			reqCtx, authority, ownerName); err != nil {
			return systemaccount.CredentialIntent{}, newSystemAccountRestoreContractError(
				systemaccount.TargetSemanticUnavailableReason, err)
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
		ResolvedSource: objectIdentity(authority.source),
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
	return r.resolveSystemAccountRestoreAuthorityWithMode(
		reqCtx, pvc, backup, clusterName, systemAccountRestoreAuthorityForRequest)
}

func (r *VolumePopulatorReconciler) resolveSystemAccountRestoreAuthorityWithMode(
	reqCtx intctrlutil.RequestCtx,
	pvc *corev1.PersistentVolumeClaim,
	backup *dpv1alpha1.Backup,
	clusterName string,
	mode systemAccountRestoreAuthorityMode,
) (*systemAccountRestoreAuthority, error) {
	if pvc == nil || backup == nil {
		return nil, fmt.Errorf("PVC and Backup are required")
	}
	if pvc.Namespace == "" || pvc.UID == "" {
		return nil, fmt.Errorf("PVC identity must include namespace and UID")
	}
	reader, err := r.systemAccountAuthorityReader()
	if err != nil {
		return nil, err
	}
	livePVC := &corev1.PersistentVolumeClaim{}
	if err := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), livePVC); err != nil {
		return nil, systemAccountAuthorityReadError(err)
	}
	if livePVC.UID != pvc.UID {
		return nil, fmt.Errorf("pvc %s/%s UID changed from %s to %s",
			pvc.Namespace, pvc.Name, pvc.UID, livePVC.UID)
	}
	if !livePVC.DeletionTimestamp.IsZero() {
		return nil, fmt.Errorf("PVC %s/%s is terminating", livePVC.Namespace, livePVC.Name)
	}
	pvc = livePVC
	liveBackup := &dpv1alpha1.Backup{}
	if err := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(backup), liveBackup); err != nil {
		return nil, systemAccountAuthorityReadError(err)
	}
	if backup.UID == "" || liveBackup.UID != backup.UID {
		return nil, &systemAccountRestoreAuthorityReadError{
			cause: fmt.Errorf("backup %s/%s UID changed from %s to %s",
				backup.Namespace, backup.Name, backup.UID, liveBackup.UID),
		}
	}
	if backup.Annotations[constant.EncryptedSystemAccountsAnnotationKey] !=
		liveBackup.Annotations[constant.EncryptedSystemAccountsAnnotationKey] {
		return nil, &systemAccountRestoreAuthorityReadError{
			cause: fmt.Errorf("backup %s/%s system account payload changed during authority resolution",
				backup.Namespace, backup.Name),
		}
	}
	source := systemaccount.SourceIdentity{
		APIGroup:  dptypes.DataprotectionAPIGroup,
		Kind:      dptypes.BackupKind,
		Namespace: liveBackup.Namespace,
		Name:      liveBackup.Name,
	}
	if !pvcReferencesRestoreSource(pvc, source) {
		return nil, newSystemAccountRestoreContractError(
			systemaccount.RestoreIntentMismatchReason,
			fmt.Errorf("PVC %s/%s declared restore source does not match Backup %s/%s",
				pvc.Namespace, pvc.Name, liveBackup.Namespace, liveBackup.Name))
	}
	workload, err := r.resolveRestorePVCWorkload(reqCtx, reader, pvc)
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
	if err := reader.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      componentRef.Name,
	}, component); err != nil {
		return nil, systemAccountAuthorityReadError(err)
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
	if err := reader.Get(reqCtx.Ctx, client.ObjectKey{
		Namespace: pvc.Namespace,
		Name:      clusterRef.Name,
	}, cluster); err != nil {
		return nil, systemAccountAuthorityReadError(err)
	}
	if cluster.UID == "" || cluster.UID != clusterRef.UID {
		return nil, fmt.Errorf("component %s/%s Cluster owner UID mismatch", component.Namespace, component.Name)
	}
	if !cluster.DeletionTimestamp.IsZero() &&
		mode != systemAccountRestoreAuthorityForConflictProjection {
		return nil, fmt.Errorf("component %s/%s Cluster owner is terminating", component.Namespace, component.Name)
	}
	if workload.GetLabels()[constant.AppInstanceLabelKey] != cluster.Name ||
		pvc.Labels[constant.AppInstanceLabelKey] != cluster.Name {
		return nil, fmt.Errorf("PVC %s/%s workload authority does not match Cluster %s/%s",
			pvc.Namespace, pvc.Name, cluster.Namespace, cluster.Name)
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
		if pvc.Annotations[constant.RestorePITRAnnotationKey] != cluster.Spec.Restore.PITR {
			return nil, newSystemAccountRestoreContractError(
				systemaccount.RestoreIntentMismatchReason,
				fmt.Errorf("PVC %s/%s PITR %q does not match Cluster restore PITR %q",
					pvc.Namespace, pvc.Name,
					pvc.Annotations[constant.RestorePITRAnnotationKey],
					cluster.Spec.Restore.PITR))
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
		pvc:       pvc,
		root:      cluster,
		component: component,
		workload:  workload,
		source:    liveBackup,
		operation: operation,
	}, nil
}

func (r *VolumePopulatorReconciler) resolveRestorePVCWorkload(
	reqCtx intctrlutil.RequestCtx,
	reader client.Reader,
	pvc *corev1.PersistentVolumeClaim,
) (*workloads.InstanceSet, error) {
	ownerRef := metav1.GetControllerOf(pvc)
	if ownerRef == nil || ownerRef.APIVersion != workloads.GroupVersion.String() {
		return nil, fmt.Errorf("PVC %s/%s has no supported workload controller owner", pvc.Namespace, pvc.Name)
	}
	switch ownerRef.Kind {
	case workloads.InstanceSetKind:
		workload := &workloads.InstanceSet{}
		if err := reader.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      ownerRef.Name,
		}, workload); err != nil {
			return nil, systemAccountAuthorityReadError(err)
		}
		if workload.UID == "" || workload.UID != ownerRef.UID {
			return nil, fmt.Errorf("PVC %s/%s workload owner UID mismatch", pvc.Namespace, pvc.Name)
		}
		if !workload.DeletionTimestamp.IsZero() {
			return nil, fmt.Errorf("PVC %s/%s workload owner is terminating", pvc.Namespace, pvc.Name)
		}
		return workload, nil
	case "Instance":
		instance := &workloads.Instance{}
		if err := reader.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      ownerRef.Name,
		}, instance); err != nil {
			return nil, systemAccountAuthorityReadError(err)
		}
		if instance.UID == "" || instance.UID != ownerRef.UID {
			return nil, fmt.Errorf("PVC %s/%s Instance owner UID mismatch", pvc.Namespace, pvc.Name)
		}
		if !instance.DeletionTimestamp.IsZero() {
			return nil, fmt.Errorf("PVC %s/%s Instance owner is terminating", pvc.Namespace, pvc.Name)
		}
		parentRef := metav1.GetControllerOf(instance)
		if parentRef == nil || parentRef.APIVersion != workloads.GroupVersion.String() ||
			parentRef.Kind != workloads.InstanceSetKind {
			return nil, fmt.Errorf("PVC %s/%s Instance owner has no parent InstanceSet controller owner",
				pvc.Namespace, pvc.Name)
		}
		workload := &workloads.InstanceSet{}
		if err := reader.Get(reqCtx.Ctx, client.ObjectKey{
			Namespace: pvc.Namespace,
			Name:      parentRef.Name,
		}, workload); err != nil {
			return nil, systemAccountAuthorityReadError(err)
		}
		if workload.UID == "" || workload.UID != parentRef.UID {
			return nil, fmt.Errorf("PVC %s/%s InstanceSet owner UID mismatch", pvc.Namespace, pvc.Name)
		}
		if !workload.DeletionTimestamp.IsZero() {
			return nil, fmt.Errorf("PVC %s/%s InstanceSet owner is terminating", pvc.Namespace, pvc.Name)
		}
		return workload, nil
	default:
		return nil, fmt.Errorf("PVC %s/%s has unsupported workload owner kind %q",
			pvc.Namespace, pvc.Name, ownerRef.Kind)
	}
}

func (r *VolumePopulatorReconciler) validateSystemAccountShardingAuthority(
	reqCtx intctrlutil.RequestCtx,
	authority *systemAccountRestoreAuthority,
	shardingName string,
) error {
	pvc := authority.pvc
	if pvc.Labels[constant.KBAppShardingNameLabelKey] != shardingName ||
		authority.workload.Labels[constant.KBAppShardingNameLabelKey] != shardingName ||
		authority.component.Labels[constant.KBAppShardingNameLabelKey] != shardingName {
		return fmt.Errorf("PVC %s/%s sharding semantics do not match selector %q",
			pvc.Namespace, pvc.Name, shardingName)
	}
	if !slices.ContainsFunc(authority.root.Spec.Shardings, func(item appsv1.ClusterSharding) bool {
		return item.Name == shardingName
	}) {
		return fmt.Errorf("cluster %s/%s has no sharding %q",
			authority.root.Namespace, authority.root.Name, shardingName)
	}
	reader, err := r.systemAccountAuthorityReader()
	if err != nil {
		return err
	}
	members, err := sharding.ListShardingComponents(
		reqCtx.Ctx, reader, authority.root, shardingName)
	if err != nil {
		return systemAccountAuthorityReadError(err)
	}
	if !slices.ContainsFunc(members, func(member appsv1.Component) bool {
		return member.Name == authority.component.Name &&
			member.UID == authority.component.UID &&
			member.DeletionTimestamp.IsZero()
	}) {
		return fmt.Errorf("component %s/%s UID %s is not in current sharding %q member set",
			authority.component.Namespace, authority.component.Name,
			authority.component.UID, shardingName)
	}
	return nil
}

func pvcReferencesRestoreSource(
	pvc *corev1.PersistentVolumeClaim,
	source systemaccount.SourceIdentity,
) bool {
	if pvc == nil || pvc.Spec.DataSourceRef == nil || pvc.Spec.DataSourceRef.APIGroup == nil ||
		*pvc.Spec.DataSourceRef.APIGroup != source.APIGroup ||
		pvc.Spec.DataSourceRef.Kind != source.Kind ||
		pvc.Spec.DataSourceRef.Name != source.Name {
		return false
	}
	namespace := pvc.Namespace
	if pvc.Spec.DataSourceRef.Namespace != nil {
		namespace = *pvc.Spec.DataSourceRef.Namespace
	}
	return namespace == source.Namespace
}

func (r *VolumePopulatorReconciler) systemAccountAuthorityReader() (client.Reader, error) {
	if r.APIReader == nil {
		return nil, &systemAccountRestoreAuthorityReadError{
			cause: fmt.Errorf("system account restore authority API reader is not configured"),
		}
	}
	return r.APIReader, nil
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
	reader, err := r.systemAccountAuthorityReader()
	if err != nil {
		return err
	}
	if err := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), current); err != nil {
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
	if getErr := reader.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), fresh); getErr == nil &&
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
	reader, err := r.systemAccountAuthorityReader()
	if err != nil {
		return "", err
	}
	state, err := systemaccount.ReadRestoreOperationState(reqCtx.Ctx, reader, operation)
	if err != nil {
		return "", err
	}
	switch state {
	case systemaccount.RestoreOperationActive:
		return systemAccountRestoreOperationActive, nil
	case systemaccount.RestoreOperationSucceeded:
		return systemAccountRestoreOperationSucceeded, nil
	case systemaccount.RestoreOperationFailed:
		return systemAccountRestoreOperationFailed, nil
	case systemaccount.RestoreOperationTerminating:
		return systemAccountRestoreOperationTerminating, nil
	case systemaccount.RestoreOperationGone:
		return systemAccountRestoreOperationGone, nil
	default:
		return "", fmt.Errorf("unsupported system account restore operation state %q", state)
	}
}

type systemAccountRestoreOperationState string

const (
	systemAccountRestoreOperationActive      systemAccountRestoreOperationState = "Active"
	systemAccountRestoreOperationSucceeded   systemAccountRestoreOperationState = "Succeeded"
	systemAccountRestoreOperationFailed      systemAccountRestoreOperationState = "Failed"
	systemAccountRestoreOperationTerminating systemAccountRestoreOperationState = "Terminating"
	systemAccountRestoreOperationGone        systemAccountRestoreOperationState = "Gone"
)
