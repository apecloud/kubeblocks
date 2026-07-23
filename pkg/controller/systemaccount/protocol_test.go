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

package systemaccount

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestLogicalTargetSlotIsIndependentFromRestoreOperation(t *testing.T) {
	first := testCredentialIntent()
	second := testCredentialIntent()
	second.Operation.Source.Name = "backup-2"
	second.Operation.PITR = "2026-07-23T10:00:00Z"
	second.ResolvedSource.UID = types.UID("backup-uid-2")

	firstRequest, err := BuildRestoreRequest(first)
	require.NoError(t, err)
	secondRequest, err := BuildRestoreRequest(second)
	require.NoError(t, err)

	require.Equal(t, firstRequest.Name, secondRequest.Name)
	require.Equal(t,
		firstRequest.Annotations[LogicalTargetDigestAnnotationKey],
		secondRequest.Annotations[LogicalTargetDigestAnnotationKey])
	require.NotEqual(t,
		firstRequest.Annotations[RestoreOperationDigestAnnotationKey],
		secondRequest.Annotations[RestoreOperationDigestAnnotationKey])
	require.NotEqual(t,
		firstRequest.Annotations[CredentialIntentRevisionAnnotationKey],
		secondRequest.Annotations[CredentialIntentRevisionAnnotationKey])
	require.Len(t, firstRequest.Name, 55)
}

func TestCanonicalOperationEncodingDistinguishesNilAndEmptyParameters(t *testing.T) {
	nilParameters := testCredentialIntent()
	nilParameters.Operation.Parameters = nil
	emptyParameters := testCredentialIntent()
	emptyParameters.Operation.Parameters = map[string]string{}

	nilDigest, err := OperationDigest(nilParameters.Operation)
	require.NoError(t, err)
	emptyDigest, err := OperationDigest(emptyParameters.Operation)
	require.NoError(t, err)

	require.NotEqual(t, nilDigest, emptyDigest)
}

func TestBuildRestoreRequestSealsProtocolEnvelope(t *testing.T) {
	intent := testCredentialIntent()

	request, err := BuildRestoreRequest(intent)
	require.NoError(t, err)
	require.Equal(t, RestoreProtocolV2, request.Annotations[RestoreProtocolAnnotationKey])
	require.Equal(t, string(RestoreRequestPhasePending), request.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, "true", request.Labels[constant.SystemAccountRestoreRequestLabelKey])
	require.NotNil(t, request.Immutable)
	require.True(t, *request.Immutable)
	require.Contains(t, request.Finalizers, RestoreProtocolFinalizer)
	require.NotContains(t, request.Annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	require.NotContains(t, request.Annotations, constant.SystemAccountProvisionedAnnotationKey)
	require.NotContains(t, request.Data, RestoreTargetEnvelopeDataKey)
	require.Equal(t, []byte("new-password"), request.Data[constant.AccountPasswdForSecret])
	require.Len(t, request.OwnerReferences, 1)
	require.Equal(t, intent.Operation.Root.UID, request.OwnerReferences[0].UID)
	require.NotNil(t, request.OwnerReferences[0].Controller)
	require.False(t, *request.OwnerReferences[0].Controller)
	require.NotNil(t, request.OwnerReferences[0].BlockOwnerDeletion)
	require.False(t, *request.OwnerReferences[0].BlockOwnerDeletion)

	parsed, err := ValidateRestoreRequestV2(request)
	require.NoError(t, err)
	require.Equal(t, intent.Operation.Root, parsed.Operation.Root)
	require.Equal(t, intent.Target, parsed.Target)
	require.Equal(t, intent.ResolvedSource, parsed.ResolvedSource)

	t.Run("metadata mirror tampering fails closed", func(t *testing.T) {
		tampered := request.DeepCopy()
		tampered.Annotations[TargetOwnerUIDAnnotationKey] = "replacement-owner"
		_, err := ValidateRestoreRequestV2(tampered)
		require.ErrorContains(t, err, "metadata mirror")

		envelope, err := DecodeRestoreIntentEnvelope(tampered)
		require.NoError(t, err)
		require.Equal(t, intent.Operation.Root, envelope.Operation.Root)
	})

	t.Run("immutable envelope tampering fails closed", func(t *testing.T) {
		tampered := request.DeepCopy()
		tampered.Data[RestoreIntentEnvelopeDataKey] = []byte(`{"protocol":"sar/v2"}`)
		_, err := ValidateRestoreRequestV2(tampered)
		require.Error(t, err)
	})
}

func TestCredentialRevisionChangesWithoutMovingLogicalTargetSlot(t *testing.T) {
	first := testCredentialIntent()
	second := testCredentialIntent()
	second.ResolvedSource.UID = types.UID("replacement-backup-uid")
	second.Credentials[constant.AccountPasswdForSecret] = []byte("rotated-password")

	firstRequest, err := BuildRestoreRequest(first)
	require.NoError(t, err)
	secondRequest, err := BuildRestoreRequest(second)
	require.NoError(t, err)

	require.Equal(t, firstRequest.Name, secondRequest.Name)
	require.Equal(t,
		firstRequest.Annotations[RestoreOperationDigestAnnotationKey],
		secondRequest.Annotations[RestoreOperationDigestAnnotationKey])
	require.NotEqual(t,
		firstRequest.Annotations[CredentialIntentRevisionAnnotationKey],
		secondRequest.Annotations[CredentialIntentRevisionAnnotationKey])
}

func TestTransitionRestoreRequestV2Table(t *testing.T) {
	phases := []RestoreRequestPhase{
		RestoreRequestPhasePending,
		RestoreRequestPhaseClaimed,
		RestoreRequestPhaseCommitted,
		RestoreRequestPhaseFailed,
		"",
		"Unknown",
	}
	allowed := map[string]bool{
		"Pending/Pending":     true,
		"Pending/Claimed":     true,
		"Pending/Failed":      true,
		"Claimed/Claimed":     true,
		"Claimed/Committed":   true,
		"Claimed/Failed":      true,
		"Committed/Committed": true,
		"Committed/Failed":    true,
		"Failed/Failed":       true,
	}
	receipt := &RestoreCommitReceipt{
		TargetName:     "target",
		TargetUID:      "target-uid",
		CommitRevision: "revision",
	}
	for _, current := range phases {
		for _, next := range phases {
			t.Run(fmt.Sprintf("%s-to-%s", current, next), func(t *testing.T) {
				request, err := BuildRestoreRequest(testCredentialIntent())
				require.NoError(t, err)
				request.Annotations[RestoreRequestPhaseAnnotationKey] = string(current)
				reason := ""
				if next == RestoreRequestPhaseFailed {
					reason = OperationTerminalReason
				}
				var commit *RestoreCommitReceipt
				if next == RestoreRequestPhaseCommitted {
					commit = receipt
				}
				updated, err := TransitionRestoreRequestV2(request, next, reason, commit, false)
				if !allowed[string(current)+"/"+string(next)] {
					require.Error(t, err)
					return
				}
				require.NoError(t, err)
				require.Equal(t, string(next), updated.Annotations[RestoreRequestPhaseAnnotationKey])
				require.Contains(t, updated.Finalizers, RestoreProtocolFinalizer)
			})
		}
	}

	t.Run("active phase cannot release finalizer", func(t *testing.T) {
		request, err := BuildRestoreRequest(testCredentialIntent())
		require.NoError(t, err)
		_, err = TransitionRestoreRequestV2(
			request, RestoreRequestPhasePending, "", nil, true)
		require.ErrorContains(t, err, "cannot release")
	})

	t.Run("failed phase releases finalizer with stable reason", func(t *testing.T) {
		request, err := BuildRestoreRequest(testCredentialIntent())
		require.NoError(t, err)
		updated, err := TransitionRestoreRequestV2(
			request, RestoreRequestPhaseFailed, RootUnavailableReason, nil, true)
		require.NoError(t, err)
		require.NotContains(t, updated.Finalizers, RestoreProtocolFinalizer)
		require.Equal(t, RootUnavailableReason,
			updated.Annotations[RestoreRequestReasonAnnotationKey])
	})
}

func TestConflictReceiptKeepsFirstBlockingSnapshot(t *testing.T) {
	loser := testCredentialIntent()
	winner := testCredentialIntent()
	winner.Operation.Source.Name = "winner-backup"
	winnerRequest, err := BuildRestoreRequest(winner)
	require.NoError(t, err)
	winnerRequest.UID = types.UID("winner-request-uid")
	winnerRequest.ResourceVersion = "17"
	winnerRequest.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)

	receipt, err := BuildConflictReceipt(loser, winnerRequest)
	require.NoError(t, err)
	require.Equal(t, ConflictProtocolV1, receipt.Annotations[RestoreProtocolAnnotationKey])
	require.Equal(t, ConcurrentRestoreIntentReason, receipt.Annotations[RestoreDecisionAnnotationKey])
	require.NotContains(t, receipt.Data, constant.AccountPasswdForSecret)
	require.NotNil(t, receipt.Immutable)
	require.True(t, *receipt.Immutable)
	require.Contains(t, receipt.Finalizers, RestoreProtocolFinalizer)

	_, err = ValidateConflictReceipt(receipt, loser)
	require.NoError(t, err)

	t.Run("later winner replacement does not rewrite historical snapshot", func(t *testing.T) {
		replacementWinner := winnerRequest.DeepCopy()
		replacementWinner.UID = types.UID("winner-request-uid-2")
		replacementWinner.ResourceVersion = "42"
		replacementWinner.Annotations[RestoreOperationDigestAnnotationKey] = "different-winner-digest"

		parsed, err := ValidateConflictReceipt(receipt, loser)
		require.NoError(t, err)
		require.Equal(t, winnerRequest.UID, parsed.BlockingRequest.UID)
		require.NotEqual(t, replacementWinner.UID, parsed.BlockingRequest.UID)
		require.Equal(t, "17", parsed.BlockingRequest.ResourceVersion)
	})

	t.Run("different current contender cannot reuse receipt", func(t *testing.T) {
		otherLoser := testCredentialIntent()
		otherLoser.Operation.PITR = "other-pitr"
		_, err := ValidateConflictReceipt(receipt, otherLoser)
		require.ErrorContains(t, err, "current contender")
	})

	t.Run("invalid sealed blocking phase fails closed", func(t *testing.T) {
		tampered := receipt.DeepCopy()
		envelope, err := DecodeConflictEnvelope(tampered)
		require.NoError(t, err)
		envelope.BlockingRequest.Phase = RestoreRequestPhaseFailed
		require.NoError(t, EncodeConflictEnvelope(tampered, envelope))
		tampered.Annotations[ObservedRequestPhaseAnnotationKey] = string(RestoreRequestPhaseFailed)
		_, err = ValidateConflictReceipt(tampered, loser)
		require.ErrorContains(t, err, "blocking phase")
	})
}

func TestClearStaleTargetRestoreReceiptPreservesMetadataOnlyChanges(t *testing.T) {
	intent := testCredentialIntent()
	request, err := BuildRestoreRequest(intent)
	require.NoError(t, err)
	request.UID = "request-uid"
	controller := true
	blockOwnerDeletion := true
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  intent.Target.Namespace,
			Name:       "cluster-mysql-admin",
			UID:        "target-uid",
			Finalizers: []string{constant.DBComponentFinalizerName},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         intent.Target.Owner.APIVersion,
				Kind:               intent.Target.Owner.Kind,
				Name:               intent.Target.Owner.Name,
				UID:                intent.Target.Owner.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte(intent.Target.Account),
			constant.AccountPasswdForSecret: []byte("new-password"),
		},
	}
	applyTargetReceipt(target, request)
	revision, err := TargetCommitRevision(target, constant.DBComponentFinalizerName)
	require.NoError(t, err)
	target.Annotations[TargetCommitRevisionAnnotationKey] = revision

	target.Labels = map[string]string{"metadata-only": "changed"}
	require.False(t, ClearStaleTargetRestoreReceipt(target, constant.DBComponentFinalizerName))
	require.Equal(t, revision, target.Annotations[TargetCommitRevisionAnnotationKey])
	require.True(t, TargetReceiptExactV2(target, request, constant.DBComponentFinalizerName))

	tamperedProtocol := target.DeepCopy()
	delete(tamperedProtocol.Annotations, RestoreProtocolAnnotationKey)
	tamperedRevision, err := TargetCommitRevision(tamperedProtocol, constant.DBComponentFinalizerName)
	require.NoError(t, err)
	require.NotEqual(t, revision, tamperedRevision)
	require.False(t, TargetReceiptExactV2(
		tamperedProtocol, request, constant.DBComponentFinalizerName))
	require.True(t, ClearStaleTargetRestoreReceipt(
		tamperedProtocol, constant.DBComponentFinalizerName))

	target.Data[constant.AccountPasswdForSecret] = []byte("rotated-password")
	require.True(t, ClearStaleTargetRestoreReceipt(target, constant.DBComponentFinalizerName))
	require.NotContains(t, target.Annotations, RestoreOperationDigestAnnotationKey)
	require.NotContains(t, target.Annotations, CredentialIntentRevisionAnnotationKey)
	require.NotContains(t, target.Annotations, TargetCommitRevisionAnnotationKey)
	require.NotContains(t, target.Annotations, RestoreRequestNameAnnotationKey)
	require.NotContains(t, target.Annotations, RestoreRequestUIDAnnotationKey)
}

func testCredentialIntent() CredentialIntent {
	return CredentialIntent{
		Operation: RestoreOperationIdentity{
			Protocol: RestoreProtocolV2,
			Profile:  RestoreProfileInitialCluster,
			Root: ObjectIdentity{
				APIVersion: "apps.kubeblocks.io/v1",
				Kind:       "Cluster",
				Namespace:  "default",
				Name:       "cluster",
				UID:        types.UID("cluster-uid"),
			},
			Source: SourceIdentity{
				APIGroup:  "dataprotection.kubeblocks.io",
				Kind:      "Backup",
				Namespace: "backup",
				Name:      "backup-1",
			},
			PITR:       "",
			Parameters: map[string]string{"restore": "all"},
		},
		Target: LogicalTargetIdentity{
			Protocol:  LogicalTargetProtocolV1,
			Namespace: "default",
			Root: ObjectIdentity{
				APIVersion: "apps.kubeblocks.io/v1",
				Kind:       "Cluster",
				Namespace:  "default",
				Name:       "cluster",
				UID:        types.UID("cluster-uid"),
			},
			Owner: ObjectIdentity{
				APIVersion: "apps.kubeblocks.io/v1",
				Kind:       "Component",
				Namespace:  "default",
				Name:       "cluster-mysql",
				UID:        types.UID("component-uid"),
			},
			Scope:   SystemAccountScopeComponent,
			Account: "root",
		},
		ResolvedSource: ObjectIdentity{
			APIVersion: "dataprotection.kubeblocks.io/v1alpha1",
			Kind:       "Backup",
			Namespace:  "backup",
			Name:       "backup-1",
			UID:        types.UID("backup-uid-1"),
		},
		Credentials: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: []byte("new-password"),
		},
	}
}

func TestRequestOwnerReferenceRequiresExactNonBlockingRoot(t *testing.T) {
	request, err := BuildRestoreRequest(testCredentialIntent())
	require.NoError(t, err)
	controller := true
	request.OwnerReferences[0].Controller = &controller

	_, err = ValidateRestoreRequestV2(request)
	require.ErrorContains(t, err, "ownerReference")
}

func TestRestoreRequestPhaseValidation(t *testing.T) {
	for _, phase := range []RestoreRequestPhase{
		RestoreRequestPhasePending,
		RestoreRequestPhaseClaimed,
		RestoreRequestPhaseCommitted,
		RestoreRequestPhaseFailed,
	} {
		require.True(t, phase.Valid())
	}
	require.False(t, RestoreRequestPhase("").Valid())
	require.False(t, RestoreRequestPhase("Unknown").Valid())
}

func TestTargetOwnerIdentityCanRepresentShardingScope(t *testing.T) {
	intent := testCredentialIntent()
	intent.Target.Scope = SystemAccountScopeSharding
	intent.Target.ShardingName = "shard"
	intent.Target.Owner = intent.Target.Root
	request, err := BuildRestoreRequest(intent)
	require.NoError(t, err)
	require.Equal(t, "shard", request.Annotations[ShardingNameAnnotationKey])
}

var _ = metav1.OwnerReference{}
var _ = corev1.Secret{}
