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
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/apecloud/kubeblocks/pkg/constant"
)

const (
	RestoreProtocolV2       = "sar/v2"
	ConflictProtocolV1      = "sar-conflict/v1"
	LogicalTargetProtocolV1 = "sar-target/v1"

	RestoreProfileInitialCluster = "initial-cluster"
	RestoreProfileLegacyPVCGroup = "legacy-pvc-group"

	SystemAccountScopeComponent = "component"
	SystemAccountScopeSharding  = "sharding"

	RestoreProtocolAnnotationKey          = "apps.kubeblocks.io/restore-protocol"
	RestoreProfileAnnotationKey           = "apps.kubeblocks.io/restore-profile"
	LogicalTargetDigestAnnotationKey      = "apps.kubeblocks.io/logical-target-digest"
	RestoreOperationDigestAnnotationKey   = "apps.kubeblocks.io/restore-operation-digest"
	RootClusterNamespaceAnnotationKey     = "apps.kubeblocks.io/root-cluster-namespace"
	RootClusterNameAnnotationKey          = "apps.kubeblocks.io/root-cluster-name"
	RootClusterUIDAnnotationKey           = "apps.kubeblocks.io/root-cluster-uid"
	SourceAPIVersionAnnotationKey         = "apps.kubeblocks.io/restore-source-api-version"
	SourceKindAnnotationKey               = "apps.kubeblocks.io/restore-source-kind"
	SourceNamespaceAnnotationKey          = "apps.kubeblocks.io/restore-source-namespace"
	SourceNameAnnotationKey               = "apps.kubeblocks.io/restore-source-name"
	SourceUIDAnnotationKey                = "apps.kubeblocks.io/restore-source-uid"
	TargetOwnerAPIVersionAnnotationKey    = "apps.kubeblocks.io/target-owner-api-version"
	TargetOwnerKindAnnotationKey          = "apps.kubeblocks.io/target-owner-kind"
	TargetOwnerNamespaceAnnotationKey     = "apps.kubeblocks.io/target-owner-namespace"
	TargetOwnerNameAnnotationKey          = "apps.kubeblocks.io/target-owner-name"
	TargetOwnerUIDAnnotationKey           = "apps.kubeblocks.io/target-owner-uid"
	SystemAccountAnnotationKey            = "apps.kubeblocks.io/system-account"
	SystemAccountScopeAnnotationKey       = "apps.kubeblocks.io/system-account-scope"
	ShardingNameAnnotationKey             = "apps.kubeblocks.io/sharding-name"
	CredentialIntentRevisionAnnotationKey = "apps.kubeblocks.io/credential-intent-revision"
	TargetCommitRevisionAnnotationKey     = "apps.kubeblocks.io/target-commit-revision"
	TargetSecretNameAnnotationKey         = "apps.kubeblocks.io/target-secret-name"
	TargetSecretUIDAnnotationKey          = "apps.kubeblocks.io/target-secret-uid"
	RestoreRequestPhaseAnnotationKey      = "apps.kubeblocks.io/restore-request-phase"
	RestoreRequestReasonAnnotationKey     = "apps.kubeblocks.io/restore-request-reason"
	RestoreRequestNameAnnotationKey       = "apps.kubeblocks.io/restore-request-name"
	RestoreRequestUIDAnnotationKey        = "apps.kubeblocks.io/restore-request-uid"

	RestoreDecisionAnnotationKey          = "apps.kubeblocks.io/restore-decision"
	BlockingRequestNamespaceAnnotationKey = "apps.kubeblocks.io/blocking-request-namespace"
	BlockingRequestNameAnnotationKey      = "apps.kubeblocks.io/blocking-request-name"
	BlockingRequestUIDAnnotationKey       = "apps.kubeblocks.io/blocking-request-uid"
	WinnerOperationDigestAnnotationKey    = "apps.kubeblocks.io/winner-operation-digest"
	ObservedRequestPhaseAnnotationKey     = "apps.kubeblocks.io/observed-request-phase"
	ObservedRequestRVAnnotationKey        = "apps.kubeblocks.io/observed-request-resource-version"

	RestoreIntentEnvelopeDataKey   = "restore-intent.json"
	RestoreConflictEnvelopeDataKey = "restore-conflict.json"
	RestoreTargetEnvelopeDataKey   = "restore-target.json"

	RestoreProtocolFinalizer = "systemaccount.apps.kubeblocks.io/restore-protection"

	ConcurrentRestoreIntentReason         = "ConcurrentRestoreIntent"
	OperationCredentialConflictReason     = "OperationCredentialConflict"
	PreviousRestoreIntentFinalizingReason = "PreviousRestoreIntentFinalizing"
	UnauthorizedRestoreProducerReason     = "UnauthorizedRestoreProducer"
	RestoreIntentMismatchReason           = "RestoreIntentMismatch"
	UnsupportedRestoreSourceReason        = "UnsupportedRestoreSource"
	OperationTerminalReason               = "OperationTerminal"
	RootUnavailableReason                 = "RootUnavailable"
	TargetOwnerUnavailableReason          = "TargetOwnerUnavailable"
	RequestDeletionRequestedReason        = "RequestDeletionRequested"
	AccountUnavailableReason              = "AccountUnavailable"
	TargetSemanticUnavailableReason       = "TargetSemanticUnavailable"
	CredentialContinuityLostReason        = "CredentialContinuityLost"
	PostWriteCancellationReason           = "PostWriteCancellation"
	InvalidPhaseReason                    = "InvalidPhase"
)

const conflictReceiptNamePrefix = "system-account-conflict-"

type RestoreRequestPhase string

const (
	RestoreRequestPhasePending   RestoreRequestPhase = "Pending"
	RestoreRequestPhaseClaimed   RestoreRequestPhase = "Claimed"
	RestoreRequestPhaseCommitted RestoreRequestPhase = "Committed"
	RestoreRequestPhaseFailed    RestoreRequestPhase = "Failed"
)

func (p RestoreRequestPhase) Valid() bool {
	switch p {
	case RestoreRequestPhasePending, RestoreRequestPhaseClaimed,
		RestoreRequestPhaseCommitted, RestoreRequestPhaseFailed:
		return true
	default:
		return false
	}
}

func (p RestoreRequestPhase) Active() bool {
	switch p {
	case RestoreRequestPhasePending, RestoreRequestPhaseClaimed, RestoreRequestPhaseCommitted:
		return true
	default:
		return false
	}
}

type RestoreCommitReceipt struct {
	TargetName     string
	TargetUID      types.UID
	CommitRevision string
}

var restoreFailureReasons = map[string]struct{}{
	UnauthorizedRestoreProducerReason:     {},
	RestoreIntentMismatchReason:           {},
	UnsupportedRestoreSourceReason:        {},
	OperationCredentialConflictReason:     {},
	ConcurrentRestoreIntentReason:         {},
	PreviousRestoreIntentFinalizingReason: {},
	OperationTerminalReason:               {},
	RootUnavailableReason:                 {},
	TargetOwnerUnavailableReason:          {},
	RequestDeletionRequestedReason:        {},
	AccountUnavailableReason:              {},
	TargetSemanticUnavailableReason:       {},
	CredentialContinuityLostReason:        {},
	PostWriteCancellationReason:           {},
	InvalidPhaseReason:                    {},
}

func IsRestoreFailureReason(reason string) bool {
	_, ok := restoreFailureReasons[reason]
	return ok
}

// TransitionRestoreRequestV2 applies phase, reason, finalizer, and commit
// receipt changes to one resourceVersion snapshot.
func TransitionRestoreRequestV2(
	request *corev1.Secret,
	next RestoreRequestPhase,
	reason string,
	receipt *RestoreCommitReceipt,
	releaseFinalizer bool,
) (*corev1.Secret, error) {
	if request == nil {
		return nil, fmt.Errorf("restore request is nil")
	}
	current := RestoreRequestPhase(request.Annotations[RestoreRequestPhaseAnnotationKey])
	if !current.Valid() || !next.Valid() {
		return nil, fmt.Errorf("invalid restore request transition %q -> %q", current, next)
	}
	if !restoreTransitionAllowed(current, next) {
		return nil, fmt.Errorf("restore request transition %q -> %q is not allowed", current, next)
	}
	if next == RestoreRequestPhaseFailed {
		if _, ok := restoreFailureReasons[reason]; !ok {
			return nil, fmt.Errorf("unsupported restore request failure reason %q", reason)
		}
	} else if reason != "" {
		return nil, fmt.Errorf("phase %q must not carry failure reason %q", next, reason)
	}
	if next == RestoreRequestPhaseCommitted {
		if receipt == nil || receipt.TargetName == "" || receipt.TargetUID == "" ||
			receipt.CommitRevision == "" {
			return nil, fmt.Errorf("committed transition requires a complete target receipt")
		}
	}
	if releaseFinalizer && next != RestoreRequestPhaseCommitted &&
		next != RestoreRequestPhaseFailed {
		return nil, fmt.Errorf("phase %q cannot release the protocol finalizer", next)
	}

	updated := request.DeepCopy()
	if updated.Annotations == nil {
		updated.Annotations = map[string]string{}
	}
	updated.Annotations[RestoreRequestPhaseAnnotationKey] = string(next)
	if reason == "" {
		delete(updated.Annotations, RestoreRequestReasonAnnotationKey)
	} else {
		updated.Annotations[RestoreRequestReasonAnnotationKey] = reason
	}
	if next == RestoreRequestPhaseCommitted {
		updated.Annotations[TargetSecretNameAnnotationKey] = receipt.TargetName
		updated.Annotations[TargetSecretUIDAnnotationKey] = string(receipt.TargetUID)
		updated.Annotations[TargetCommitRevisionAnnotationKey] = receipt.CommitRevision
	}
	if releaseFinalizer {
		updated.Finalizers = slices.DeleteFunc(updated.Finalizers,
			func(finalizer string) bool { return finalizer == RestoreProtocolFinalizer })
	}
	return updated, nil
}

func restoreTransitionAllowed(current, next RestoreRequestPhase) bool {
	if current == next {
		return true
	}
	switch current {
	case RestoreRequestPhasePending:
		return next == RestoreRequestPhaseClaimed || next == RestoreRequestPhaseFailed
	case RestoreRequestPhaseClaimed:
		return next == RestoreRequestPhaseCommitted || next == RestoreRequestPhaseFailed
	case RestoreRequestPhaseCommitted:
		return next == RestoreRequestPhaseFailed
	default:
		return false
	}
}

type ObjectIdentity struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Namespace  string    `json:"namespace"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`
}

type SourceIdentity struct {
	APIGroup  string `json:"apiGroup"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type RestoreOperationIdentity struct {
	Protocol   string            `json:"protocol"`
	Profile    string            `json:"profile"`
	Root       ObjectIdentity    `json:"root"`
	Source     SourceIdentity    `json:"source"`
	PITR       string            `json:"pitr"`
	Parameters map[string]string `json:"parameters"`
}

type LogicalTargetIdentity struct {
	Protocol     string         `json:"protocol"`
	Namespace    string         `json:"namespace"`
	Root         ObjectIdentity `json:"root"`
	Owner        ObjectIdentity `json:"owner"`
	Scope        string         `json:"scope"`
	ShardingName string         `json:"shardingName"`
	Account      string         `json:"account"`
}

type CredentialIntent struct {
	Operation      RestoreOperationIdentity `json:"operation"`
	Target         LogicalTargetIdentity    `json:"target"`
	ResolvedSource ObjectIdentity           `json:"resolvedSource"`
	Credentials    map[string][]byte        `json:"credentials"`
}

type BlockingRequestSnapshot struct {
	Namespace             string              `json:"namespace"`
	Name                  string              `json:"name"`
	UID                   types.UID           `json:"uid"`
	ResourceVersion       string              `json:"resourceVersion"`
	Phase                 RestoreRequestPhase `json:"phase"`
	WinnerOperationDigest string              `json:"winnerOperationDigest"`
}

type ConflictEnvelope struct {
	Protocol        string                   `json:"protocol"`
	Decision        string                   `json:"decision"`
	Target          LogicalTargetIdentity    `json:"target"`
	LoserOperation  RestoreOperationIdentity `json:"loserOperation"`
	BlockingRequest BlockingRequestSnapshot  `json:"blockingRequest"`
}

func OperationDigest(identity RestoreOperationIdentity) (string, error) {
	encoded, err := encodeOperationIdentity(identity)
	if err != nil {
		return "", err
	}
	return sha256Hex(encoded), nil
}

func LogicalTargetDigest(identity LogicalTargetIdentity) (string, error) {
	encoded, err := encodeLogicalTargetIdentity(identity)
	if err != nil {
		return "", err
	}
	return sha256Hex(encoded), nil
}

func RestoreRequestNameForTarget(identity LogicalTargetIdentity) (string, error) {
	encoded, err := encodeLogicalTargetIdentity(identity)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return restoreRequestNamePrefix + hex.EncodeToString(digest[:16]), nil
}

func ConflictReceiptName(target LogicalTargetIdentity, loser RestoreOperationIdentity) (string, error) {
	targetBytes, err := encodeLogicalTargetIdentity(target)
	if err != nil {
		return "", err
	}
	operationBytes, err := encodeOperationIdentity(loser)
	if err != nil {
		return "", err
	}
	encoder := newCanonicalEncoder()
	encoder.writeVersion(ConflictProtocolV1)
	encoder.writeBytes(targetBytes)
	encoder.writeBytes(operationBytes)
	digest := sha256.Sum256(encoder.bytes())
	return conflictReceiptNamePrefix + hex.EncodeToString(digest[:16]), nil
}

func CredentialIntentRevision(intent CredentialIntent) (string, error) {
	if err := validateCredentialIntent(intent); err != nil {
		return "", err
	}
	operationBytes, _ := encodeOperationIdentity(intent.Operation)
	targetBytes, _ := encodeLogicalTargetIdentity(intent.Target)
	encoder := newCanonicalEncoder()
	encoder.writeVersion(RestoreProtocolV2)
	encoder.writeBytes(operationBytes)
	encoder.writeBytes(targetBytes)
	encoder.writeObjectIdentity(intent.ResolvedSource)
	encoder.writeBytesMap(intent.Credentials)
	return sha256Hex(encoder.bytes()), nil
}

func BuildRestoreRequest(intent CredentialIntent) (*corev1.Secret, error) {
	if err := validateCredentialIntent(intent); err != nil {
		return nil, err
	}
	operationDigest, _ := OperationDigest(intent.Operation)
	targetDigest, _ := LogicalTargetDigest(intent.Target)
	intentRevision, _ := CredentialIntentRevision(intent)
	name, _ := RestoreRequestNameForTarget(intent.Target)
	envelope, err := json.Marshal(intent)
	if err != nil {
		return nil, fmt.Errorf("marshal restore intent: %w", err)
	}
	immutable := true
	controller := false
	blockOwnerDeletion := false
	request := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: intent.Target.Namespace,
			Name:      name,
			Labels: map[string]string{
				constant.SystemAccountRestoreRequestLabelKey: "true",
			},
			Annotations: requestIdentityAnnotations(intent, operationDigest, targetDigest, intentRevision),
			Finalizers:  []string{RestoreProtocolFinalizer},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         intent.Operation.Root.APIVersion,
				Kind:               intent.Operation.Root.Kind,
				Name:               intent.Operation.Root.Name,
				UID:                intent.Operation.Root.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
		},
		Immutable: &immutable,
		Type:      corev1.SecretTypeOpaque,
		Data:      maps.Clone(intent.Credentials),
	}
	request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhasePending)
	request.Data[RestoreIntentEnvelopeDataKey] = envelope
	return request, nil
}

func ValidateRestoreRequestV2(request *corev1.Secret) (CredentialIntent, error) {
	intent, err := DecodeRestoreRequestV2(request)
	if err != nil {
		return intent, err
	}
	if !slices.Contains(request.Finalizers, RestoreProtocolFinalizer) {
		return intent, fmt.Errorf("restore request %s/%s has no protocol finalizer", request.Namespace, request.Name)
	}
	phase := RestoreRequestPhase(request.Annotations[RestoreRequestPhaseAnnotationKey])
	if !phase.Valid() {
		return intent, fmt.Errorf("restore request %s/%s has invalid phase %q", request.Namespace, request.Name, phase)
	}
	return intent, nil
}

// DecodeRestoreRequestV2 validates the immutable request identity and payload
// while allowing lifecycle cleanup to inspect unknown phases or a request whose
// protocol finalizer has already been released.
func DecodeRestoreRequestV2(request *corev1.Secret) (CredentialIntent, error) {
	var intent CredentialIntent
	if request == nil {
		return intent, fmt.Errorf("restore request is nil")
	}
	if request.Annotations[RestoreProtocolAnnotationKey] != RestoreProtocolV2 {
		return intent, fmt.Errorf("restore request %s/%s has unsupported protocol", request.Namespace, request.Name)
	}
	if request.Labels[constant.SystemAccountRestoreRequestLabelKey] != "true" {
		return intent, fmt.Errorf("restore request %s/%s has no request label", request.Namespace, request.Name)
	}
	if request.Immutable == nil || !*request.Immutable {
		return intent, fmt.Errorf("restore request %s/%s is not immutable", request.Namespace, request.Name)
	}
	intent, err := DecodeRestoreIntentEnvelope(request)
	if err != nil {
		return intent, err
	}
	name, _ := RestoreRequestNameForTarget(intent.Target)
	if request.Namespace != intent.Target.Namespace || request.Name != name {
		return intent, fmt.Errorf("restore request %s/%s has a non-canonical logical-target name", request.Namespace, request.Name)
	}
	operationDigest, _ := OperationDigest(intent.Operation)
	targetDigest, _ := LogicalTargetDigest(intent.Target)
	intentRevision, _ := CredentialIntentRevision(intent)
	expectedAnnotations := requestIdentityAnnotations(intent, operationDigest, targetDigest, intentRevision)
	if err := validateAnnotationMirrors(request.Annotations, expectedAnnotations); err != nil {
		return intent, fmt.Errorf("restore request %s/%s metadata mirror: %w", request.Namespace, request.Name, err)
	}
	if !reflect.DeepEqual(credentialsFromRequest(request.Data), intent.Credentials) {
		return intent, fmt.Errorf("restore request %s/%s credential payload differs from envelope", request.Namespace, request.Name)
	}
	if err := validateRootOwnerReference(request.OwnerReferences, intent.Operation.Root); err != nil {
		return intent, fmt.Errorf("restore request %s/%s ownerReference: %w", request.Namespace, request.Name, err)
	}
	if request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey] != "" ||
		request.Annotations[constant.SystemAccountProvisionedAnnotationKey] != "" {
		return intent, fmt.Errorf("restore request %s/%s contains Apps-owned target semantics", request.Namespace, request.Name)
	}
	return intent, nil
}

// DecodeRestoreIntentEnvelope reads only the immutable request payload. The
// lifecycle controller uses it to find the exact root after mutable metadata
// has been damaged, without accepting that metadata for active reconciliation.
func DecodeRestoreIntentEnvelope(request *corev1.Secret) (CredentialIntent, error) {
	var intent CredentialIntent
	if request == nil {
		return intent, fmt.Errorf("restore request is nil")
	}
	if request.Immutable == nil || !*request.Immutable {
		return intent, fmt.Errorf("restore request %s/%s is not immutable", request.Namespace, request.Name)
	}
	if err := json.Unmarshal(request.Data[RestoreIntentEnvelopeDataKey], &intent); err != nil {
		return intent, fmt.Errorf("decode restore request %s/%s envelope: %w", request.Namespace, request.Name, err)
	}
	if err := validateCredentialIntent(intent); err != nil {
		return intent, fmt.Errorf("restore request %s/%s envelope: %w", request.Namespace, request.Name, err)
	}
	return intent, nil
}

func RestoreConvergedV2(target, request *corev1.Secret, requiredFinalizer string) bool {
	if target == nil || request == nil || !target.DeletionTimestamp.IsZero() ||
		request.UID == "" ||
		RestoreRequestPhase(request.Annotations[RestoreRequestPhaseAnnotationKey]) != RestoreRequestPhaseCommitted {
		return false
	}
	intent, err := ValidateRestoreRequestV2(request)
	if err != nil {
		return false
	}
	if target.Namespace != intent.Target.Namespace ||
		target.Name != request.Annotations[TargetSecretNameAnnotationKey] ||
		target.UID == "" || string(target.UID) != request.Annotations[TargetSecretUIDAnnotationKey] ||
		!TargetReceiptExactV2(target, request, requiredFinalizer) {
		return false
	}
	revision, err := TargetCommitRevision(target, requiredFinalizer)
	if err != nil {
		return false
	}
	requestRevision := request.Annotations[TargetCommitRevisionAnnotationKey]
	targetRevision := target.Annotations[TargetCommitRevisionAnnotationKey]
	return revision == requestRevision && requestRevision == targetRevision
}

// TargetReceiptExactV2 verifies the live target commit without requiring the
// request to have reached Committed. This closes the target-write/request-write
// crash window for the Apps lifecycle handler.
func TargetReceiptExactV2(target, request *corev1.Secret, requiredFinalizer string) bool {
	if target == nil || request == nil || !target.DeletionTimestamp.IsZero() ||
		target.UID == "" || request.UID == "" {
		return false
	}
	intent, err := DecodeRestoreRequestV2(request)
	if err != nil {
		return false
	}
	if target.Namespace != intent.Target.Namespace ||
		target.Annotations[RestoreProtocolAnnotationKey] != RestoreProtocolV2 ||
		request.Name != target.Annotations[RestoreRequestNameAnnotationKey] ||
		string(request.UID) != target.Annotations[RestoreRequestUIDAnnotationKey] ||
		request.Annotations[RestoreOperationDigestAnnotationKey] !=
			target.Annotations[RestoreOperationDigestAnnotationKey] ||
		request.Annotations[CredentialIntentRevisionAnnotationKey] !=
			target.Annotations[CredentialIntentRevisionAnnotationKey] ||
		target.Annotations[constant.SystemAccountProvisionedAnnotationKey] != "true" ||
		!slices.Contains(target.Finalizers, requiredFinalizer) ||
		!reflect.DeepEqual(target.Data, intent.Credentials) {
		return false
	}
	controller := metav1.GetControllerOf(target)
	if controller == nil || controller.APIVersion != intent.Target.Owner.APIVersion ||
		controller.Kind != intent.Target.Owner.Kind || controller.Name != intent.Target.Owner.Name ||
		controller.UID != intent.Target.Owner.UID {
		return false
	}
	revision, err := TargetCommitRevision(target, requiredFinalizer)
	return err == nil && revision == target.Annotations[TargetCommitRevisionAnnotationKey]
}

// ClearTargetRestoreReceipt removes only sar/v2 commit receipts. Credential
// bytes and the Apps-owned provisioned marker are deliberately retained.
func ClearTargetRestoreReceipt(target *corev1.Secret) {
	if target == nil || target.Annotations == nil {
		return
	}
	delete(target.Annotations, RestoreOperationDigestAnnotationKey)
	delete(target.Annotations, CredentialIntentRevisionAnnotationKey)
	delete(target.Annotations, TargetCommitRevisionAnnotationKey)
	delete(target.Annotations, RestoreRequestNameAnnotationKey)
	delete(target.Annotations, RestoreRequestUIDAnnotationKey)
}

// ClearStaleTargetRestoreReceipt atomically invalidates protocol receipts when
// an Apps-owned sealed target field changes. Ordinary labels and annotations
// are outside the commit revision and therefore keep an exact receipt.
func ClearStaleTargetRestoreReceipt(target *corev1.Secret, requiredFinalizer string) bool {
	if target == nil || target.Annotations == nil {
		return false
	}
	hasReceipt := false
	for _, key := range []string{
		RestoreOperationDigestAnnotationKey,
		CredentialIntentRevisionAnnotationKey,
		TargetCommitRevisionAnnotationKey,
		RestoreRequestNameAnnotationKey,
		RestoreRequestUIDAnnotationKey,
	} {
		if target.Annotations[key] != "" {
			hasReceipt = true
			break
		}
	}
	if !hasReceipt {
		return false
	}
	revision, err := TargetCommitRevision(target, requiredFinalizer)
	if err == nil && revision == target.Annotations[TargetCommitRevisionAnnotationKey] {
		return false
	}
	ClearTargetRestoreReceipt(target)
	return true
}

func TargetCommitRevision(target *corev1.Secret, requiredFinalizer string) (string, error) {
	if target == nil {
		return "", fmt.Errorf("target Secret is nil")
	}
	controller := metav1.GetControllerOf(target)
	if controller == nil {
		return "", fmt.Errorf("target Secret %s/%s has no controller owner", target.Namespace, target.Name)
	}
	encoder := newCanonicalEncoder()
	encoder.writeVersion("sar-target-commit/v1")
	encoder.writeString(target.Annotations[RestoreProtocolAnnotationKey])
	encoder.writeString(target.Namespace)
	encoder.writeString(target.Name)
	encoder.writeString(string(target.Type))
	switch {
	case target.Immutable == nil:
		encoder.writeBytes(nil)
	case *target.Immutable:
		encoder.writeBytes([]byte{1})
	default:
		encoder.writeBytes([]byte{0})
	}
	encoder.writeBytesMap(target.Data)
	encoder.writeString(controller.APIVersion)
	encoder.writeString(controller.Kind)
	encoder.writeString(controller.Name)
	encoder.writeString(string(controller.UID))
	encoder.writeString(requiredFinalizer)
	if slices.Contains(target.Finalizers, requiredFinalizer) {
		encoder.writeBytes([]byte{1})
	} else {
		encoder.writeBytes([]byte{0})
	}
	encoder.writeString(target.Annotations[constant.SystemAccountProvisionedAnnotationKey])
	encoder.writeString(target.Annotations[RestoreOperationDigestAnnotationKey])
	encoder.writeString(target.Annotations[CredentialIntentRevisionAnnotationKey])
	encoder.writeString(target.Annotations[RestoreRequestNameAnnotationKey])
	encoder.writeString(target.Annotations[RestoreRequestUIDAnnotationKey])
	return sha256Hex(encoder.bytes()), nil
}

func BuildConflictReceipt(loser CredentialIntent, blockingRequest *corev1.Secret) (*corev1.Secret, error) {
	if err := validateCredentialIntent(loser); err != nil {
		return nil, err
	}
	winner, err := ValidateRestoreRequestV2(blockingRequest)
	if err != nil {
		return nil, fmt.Errorf("validate blocking request: %w", err)
	}
	loserTargetDigest, _ := LogicalTargetDigest(loser.Target)
	winnerTargetDigest, _ := LogicalTargetDigest(winner.Target)
	if loserTargetDigest != winnerTargetDigest {
		return nil, fmt.Errorf("blocking request has a different logical target")
	}
	loserDigest, _ := OperationDigest(loser.Operation)
	winnerDigest, _ := OperationDigest(winner.Operation)
	if loserDigest == winnerDigest {
		return nil, fmt.Errorf("blocking request belongs to the same restore operation")
	}
	phase := RestoreRequestPhase(blockingRequest.Annotations[RestoreRequestPhaseAnnotationKey])
	if !phase.Active() {
		return nil, fmt.Errorf("blocking phase %q is not active", phase)
	}
	if blockingRequest.UID == "" || blockingRequest.ResourceVersion == "" {
		return nil, fmt.Errorf("blocking request UID and resourceVersion must not be empty")
	}
	envelope := ConflictEnvelope{
		Protocol:       ConflictProtocolV1,
		Decision:       ConcurrentRestoreIntentReason,
		Target:         loser.Target,
		LoserOperation: loser.Operation,
		BlockingRequest: BlockingRequestSnapshot{
			Namespace:             blockingRequest.Namespace,
			Name:                  blockingRequest.Name,
			UID:                   blockingRequest.UID,
			ResourceVersion:       blockingRequest.ResourceVersion,
			Phase:                 phase,
			WinnerOperationDigest: winnerDigest,
		},
	}
	name, err := ConflictReceiptName(loser.Target, loser.Operation)
	if err != nil {
		return nil, err
	}
	targetDigest, _ := LogicalTargetDigest(loser.Target)
	annotations := conflictAnnotations(envelope, loserDigest, targetDigest)
	immutable := true
	controller := false
	blockOwnerDeletion := false
	receipt := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   loser.Target.Namespace,
			Name:        name,
			Annotations: annotations,
			Finalizers:  []string{RestoreProtocolFinalizer},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion:         loser.Operation.Root.APIVersion,
				Kind:               loser.Operation.Root.Kind,
				Name:               loser.Operation.Root.Name,
				UID:                loser.Operation.Root.UID,
				Controller:         &controller,
				BlockOwnerDeletion: &blockOwnerDeletion,
			}},
		},
		Immutable: &immutable,
		Type:      corev1.SecretTypeOpaque,
		Data:      map[string][]byte{},
	}
	if err := EncodeConflictEnvelope(receipt, envelope); err != nil {
		return nil, err
	}
	return receipt, nil
}

func ValidateConflictReceipt(receipt *corev1.Secret, contender CredentialIntent) (ConflictEnvelope, error) {
	if err := validateCredentialIntent(contender); err != nil {
		return ConflictEnvelope{}, fmt.Errorf("current contender: %w", err)
	}
	envelope, err := DecodeAndValidateConflictReceipt(receipt, true)
	if err != nil {
		return envelope, err
	}
	expectedTargetDigest, _ := LogicalTargetDigest(contender.Target)
	actualTargetDigest, _ := LogicalTargetDigest(envelope.Target)
	expectedOperationDigest, _ := OperationDigest(contender.Operation)
	actualOperationDigest, _ := OperationDigest(envelope.LoserOperation)
	if expectedTargetDigest != actualTargetDigest || expectedOperationDigest != actualOperationDigest {
		return envelope, fmt.Errorf("conflict receipt %s/%s current contender identity mismatch", receipt.Namespace, receipt.Name)
	}
	return envelope, nil
}

// DecodeAndValidateConflictReceipt validates the sealed loser decision without
// requiring credential bytes. Lifecycle cleanup may allow a finalizer that was
// already released, while active arbitration requires it.
func DecodeAndValidateConflictReceipt(
	receipt *corev1.Secret,
	requireFinalizer bool,
) (ConflictEnvelope, error) {
	var envelope ConflictEnvelope
	if receipt == nil {
		return envelope, fmt.Errorf("conflict receipt is nil")
	}
	if receipt.Annotations[RestoreProtocolAnnotationKey] != ConflictProtocolV1 ||
		receipt.Annotations[RestoreDecisionAnnotationKey] != ConcurrentRestoreIntentReason {
		return envelope, fmt.Errorf("conflict receipt %s/%s has invalid protocol or decision", receipt.Namespace, receipt.Name)
	}
	if receipt.Immutable == nil || !*receipt.Immutable {
		return envelope, fmt.Errorf("conflict receipt %s/%s is not immutable", receipt.Namespace, receipt.Name)
	}
	if requireFinalizer && !slices.Contains(receipt.Finalizers, RestoreProtocolFinalizer) {
		return envelope, fmt.Errorf("conflict receipt %s/%s has no protocol finalizer", receipt.Namespace, receipt.Name)
	}
	envelope, err := DecodeConflictEnvelope(receipt)
	if err != nil {
		return envelope, err
	}
	if envelope.Protocol != ConflictProtocolV1 || envelope.Decision != ConcurrentRestoreIntentReason {
		return envelope, fmt.Errorf("conflict receipt %s/%s envelope has invalid protocol or decision", receipt.Namespace, receipt.Name)
	}
	if _, err := encodeLogicalTargetIdentity(envelope.Target); err != nil {
		return envelope, fmt.Errorf("conflict receipt logical target: %w", err)
	}
	if _, err := encodeOperationIdentity(envelope.LoserOperation); err != nil {
		return envelope, fmt.Errorf("conflict receipt loser operation: %w", err)
	}
	actualTargetDigest, _ := LogicalTargetDigest(envelope.Target)
	actualOperationDigest, _ := OperationDigest(envelope.LoserOperation)
	if envelope.BlockingRequest.Namespace == "" || envelope.BlockingRequest.Name == "" ||
		envelope.BlockingRequest.UID == "" || envelope.BlockingRequest.ResourceVersion == "" ||
		envelope.BlockingRequest.WinnerOperationDigest == "" {
		return envelope, fmt.Errorf("conflict receipt %s/%s has incomplete blocking snapshot", receipt.Namespace, receipt.Name)
	}
	if !envelope.BlockingRequest.Phase.Active() {
		return envelope, fmt.Errorf("conflict receipt %s/%s has invalid blocking phase %q",
			receipt.Namespace, receipt.Name, envelope.BlockingRequest.Phase)
	}
	expectedAnnotations := conflictAnnotations(envelope, actualOperationDigest, actualTargetDigest)
	if err := validateAnnotationMirrors(receipt.Annotations, expectedAnnotations); err != nil {
		return envelope, fmt.Errorf("conflict receipt %s/%s metadata mirror: %w", receipt.Namespace, receipt.Name, err)
	}
	name, _ := ConflictReceiptName(envelope.Target, envelope.LoserOperation)
	if receipt.Namespace != envelope.Target.Namespace || receipt.Name != name {
		return envelope, fmt.Errorf("conflict receipt %s/%s has a non-canonical name", receipt.Namespace, receipt.Name)
	}
	if err := validateRootOwnerReference(receipt.OwnerReferences, envelope.LoserOperation.Root); err != nil {
		return envelope, fmt.Errorf("conflict receipt %s/%s ownerReference: %w", receipt.Namespace, receipt.Name, err)
	}
	return envelope, nil
}

func DecodeConflictEnvelope(receipt *corev1.Secret) (ConflictEnvelope, error) {
	var envelope ConflictEnvelope
	if receipt == nil {
		return envelope, fmt.Errorf("conflict receipt is nil")
	}
	if err := json.Unmarshal(receipt.Data[RestoreConflictEnvelopeDataKey], &envelope); err != nil {
		return envelope, fmt.Errorf("decode conflict receipt %s/%s envelope: %w", receipt.Namespace, receipt.Name, err)
	}
	return envelope, nil
}

func EncodeConflictEnvelope(receipt *corev1.Secret, envelope ConflictEnvelope) error {
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("encode conflict receipt envelope: %w", err)
	}
	if receipt.Data == nil {
		receipt.Data = map[string][]byte{}
	}
	receipt.Data[RestoreConflictEnvelopeDataKey] = encoded
	return nil
}

func requestIdentityAnnotations(intent CredentialIntent, operationDigest, targetDigest, intentRevision string) map[string]string {
	return map[string]string{
		RestoreProtocolAnnotationKey:          RestoreProtocolV2,
		RestoreProfileAnnotationKey:           intent.Operation.Profile,
		LogicalTargetDigestAnnotationKey:      targetDigest,
		RestoreOperationDigestAnnotationKey:   operationDigest,
		RootClusterNamespaceAnnotationKey:     intent.Operation.Root.Namespace,
		RootClusterNameAnnotationKey:          intent.Operation.Root.Name,
		RootClusterUIDAnnotationKey:           string(intent.Operation.Root.UID),
		SourceAPIVersionAnnotationKey:         intent.ResolvedSource.APIVersion,
		SourceKindAnnotationKey:               intent.ResolvedSource.Kind,
		SourceNamespaceAnnotationKey:          intent.ResolvedSource.Namespace,
		SourceNameAnnotationKey:               intent.ResolvedSource.Name,
		SourceUIDAnnotationKey:                string(intent.ResolvedSource.UID),
		TargetOwnerAPIVersionAnnotationKey:    intent.Target.Owner.APIVersion,
		TargetOwnerKindAnnotationKey:          intent.Target.Owner.Kind,
		TargetOwnerNamespaceAnnotationKey:     intent.Target.Owner.Namespace,
		TargetOwnerNameAnnotationKey:          intent.Target.Owner.Name,
		TargetOwnerUIDAnnotationKey:           string(intent.Target.Owner.UID),
		SystemAccountAnnotationKey:            intent.Target.Account,
		SystemAccountScopeAnnotationKey:       intent.Target.Scope,
		ShardingNameAnnotationKey:             intent.Target.ShardingName,
		CredentialIntentRevisionAnnotationKey: intentRevision,
	}
}

func conflictAnnotations(envelope ConflictEnvelope, loserDigest, targetDigest string) map[string]string {
	return map[string]string{
		RestoreProtocolAnnotationKey:          ConflictProtocolV1,
		RestoreDecisionAnnotationKey:          ConcurrentRestoreIntentReason,
		LogicalTargetDigestAnnotationKey:      targetDigest,
		RestoreOperationDigestAnnotationKey:   loserDigest,
		RootClusterUIDAnnotationKey:           string(envelope.Target.Root.UID),
		TargetOwnerUIDAnnotationKey:           string(envelope.Target.Owner.UID),
		BlockingRequestNamespaceAnnotationKey: envelope.BlockingRequest.Namespace,
		BlockingRequestNameAnnotationKey:      envelope.BlockingRequest.Name,
		BlockingRequestUIDAnnotationKey:       string(envelope.BlockingRequest.UID),
		WinnerOperationDigestAnnotationKey:    envelope.BlockingRequest.WinnerOperationDigest,
		ObservedRequestPhaseAnnotationKey:     string(envelope.BlockingRequest.Phase),
		ObservedRequestRVAnnotationKey:        envelope.BlockingRequest.ResourceVersion,
	}
}

func validateCredentialIntent(intent CredentialIntent) error {
	if _, err := encodeOperationIdentity(intent.Operation); err != nil {
		return err
	}
	if _, err := encodeLogicalTargetIdentity(intent.Target); err != nil {
		return err
	}
	if err := validateObjectIdentity("resolved source", intent.ResolvedSource); err != nil {
		return err
	}
	if intent.Operation.Root != intent.Target.Root {
		return fmt.Errorf("operation root and logical-target root differ")
	}
	if len(intent.Credentials) == 0 {
		return fmt.Errorf("credential payload must not be empty")
	}
	if len(intent.Credentials[constant.AccountNameForSecret]) == 0 ||
		len(intent.Credentials[constant.AccountPasswdForSecret]) == 0 {
		return fmt.Errorf("credential payload requires account name and password")
	}
	if string(intent.Credentials[constant.AccountNameForSecret]) != intent.Target.Account {
		return fmt.Errorf("credential account differs from logical target account")
	}
	return nil
}

func encodeOperationIdentity(identity RestoreOperationIdentity) ([]byte, error) {
	if identity.Protocol != RestoreProtocolV2 {
		return nil, fmt.Errorf("unsupported restore protocol %q", identity.Protocol)
	}
	if identity.Profile != RestoreProfileInitialCluster && identity.Profile != RestoreProfileLegacyPVCGroup {
		return nil, fmt.Errorf("unsupported restore profile %q", identity.Profile)
	}
	if err := validateObjectIdentity("root", identity.Root); err != nil {
		return nil, err
	}
	if identity.Root.Kind != "Cluster" {
		return nil, fmt.Errorf("root kind must be Cluster")
	}
	if identity.Source.APIGroup == "" || identity.Source.Kind == "" ||
		identity.Source.Namespace == "" || identity.Source.Name == "" {
		return nil, fmt.Errorf("restore source identity must be complete")
	}
	encoder := newCanonicalEncoder()
	encoder.writeVersion(identity.Protocol)
	encoder.writeString(identity.Profile)
	encoder.writeObjectIdentity(identity.Root)
	encoder.writeString(identity.Source.APIGroup)
	encoder.writeString(identity.Source.Kind)
	encoder.writeString(identity.Source.Namespace)
	encoder.writeString(identity.Source.Name)
	encoder.writeString(identity.PITR)
	encoder.writeStringMap(identity.Parameters)
	return encoder.bytes(), nil
}

func encodeLogicalTargetIdentity(identity LogicalTargetIdentity) ([]byte, error) {
	if identity.Protocol != LogicalTargetProtocolV1 {
		return nil, fmt.Errorf("unsupported logical-target protocol %q", identity.Protocol)
	}
	if identity.Namespace == "" {
		return nil, fmt.Errorf("logical-target namespace must not be empty")
	}
	if err := validateObjectIdentity("root", identity.Root); err != nil {
		return nil, err
	}
	if err := validateObjectIdentity("target owner", identity.Owner); err != nil {
		return nil, err
	}
	if identity.Namespace != identity.Root.Namespace || identity.Namespace != identity.Owner.Namespace {
		return nil, fmt.Errorf("logical target, root, and target owner must share a namespace")
	}
	switch identity.Scope {
	case SystemAccountScopeComponent:
		if identity.Owner.Kind != "Component" {
			return nil, fmt.Errorf("component scope requires a Component target owner")
		}
		if identity.ShardingName != "" {
			return nil, fmt.Errorf("component scope must not carry a sharding name")
		}
	case SystemAccountScopeSharding:
		if identity.Owner != identity.Root {
			return nil, fmt.Errorf("sharding scope requires the root Cluster as target owner")
		}
		if identity.ShardingName == "" {
			return nil, fmt.Errorf("sharding scope requires a sharding name")
		}
	default:
		return nil, fmt.Errorf("unsupported system account scope %q", identity.Scope)
	}
	if identity.Account == "" {
		return nil, fmt.Errorf("system account must not be empty")
	}
	encoder := newCanonicalEncoder()
	encoder.writeVersion(identity.Protocol)
	encoder.writeString(identity.Namespace)
	encoder.writeObjectIdentity(identity.Root)
	encoder.writeObjectIdentity(identity.Owner)
	encoder.writeString(identity.Scope)
	encoder.writeString(identity.ShardingName)
	encoder.writeString(identity.Account)
	return encoder.bytes(), nil
}

func validateObjectIdentity(field string, identity ObjectIdentity) error {
	if identity.APIVersion == "" || identity.Kind == "" || identity.Namespace == "" ||
		identity.Name == "" || identity.UID == "" {
		return fmt.Errorf("%s identity must be complete", field)
	}
	return nil
}

func validateRootOwnerReference(refs []metav1.OwnerReference, root ObjectIdentity) error {
	if len(refs) != 1 {
		return fmt.Errorf("expected exactly one root ownerReference")
	}
	ref := refs[0]
	if ref.APIVersion != root.APIVersion || ref.Kind != root.Kind ||
		ref.Name != root.Name || ref.UID != root.UID ||
		ref.Controller == nil || *ref.Controller ||
		ref.BlockOwnerDeletion == nil || *ref.BlockOwnerDeletion {
		return fmt.Errorf("root ownerReference does not match immutable root identity")
	}
	return nil
}

func validateAnnotationMirrors(actual, expected map[string]string) error {
	for key, expectedValue := range expected {
		actualValue, ok := actual[key]
		if !ok || actualValue != expectedValue {
			return fmt.Errorf("%s=%q, expected %q", key, actualValue, expectedValue)
		}
	}
	return nil
}

func credentialsFromRequest(data map[string][]byte) map[string][]byte {
	credentials := maps.Clone(data)
	delete(credentials, RestoreIntentEnvelopeDataKey)
	return credentials
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

type canonicalEncoder struct {
	buffer bytes.Buffer
}

func newCanonicalEncoder() *canonicalEncoder {
	return &canonicalEncoder{}
}

func (e *canonicalEncoder) bytes() []byte {
	return e.buffer.Bytes()
}

func (e *canonicalEncoder) writeVersion(value string) {
	_ = binary.Write(&e.buffer, binary.BigEndian, uint16(len(value)))
	_, _ = e.buffer.WriteString(value)
}

func (e *canonicalEncoder) writeString(value string) {
	_ = e.buffer.WriteByte(1)
	_ = binary.Write(&e.buffer, binary.BigEndian, uint32(len(value)))
	_, _ = e.buffer.WriteString(value)
}

func (e *canonicalEncoder) writeBytes(value []byte) {
	if value == nil {
		_ = e.buffer.WriteByte(0)
		return
	}
	_ = e.buffer.WriteByte(1)
	_ = binary.Write(&e.buffer, binary.BigEndian, uint32(len(value)))
	_, _ = e.buffer.Write(value)
}

func (e *canonicalEncoder) writeObjectIdentity(identity ObjectIdentity) {
	e.writeString(identity.APIVersion)
	e.writeString(identity.Kind)
	e.writeString(identity.Namespace)
	e.writeString(identity.Name)
	e.writeString(string(identity.UID))
}

func (e *canonicalEncoder) writeStringMap(value map[string]string) {
	if value == nil {
		_ = e.buffer.WriteByte(0)
		return
	}
	_ = e.buffer.WriteByte(1)
	keys := slices.Sorted(maps.Keys(value))
	_ = binary.Write(&e.buffer, binary.BigEndian, uint32(len(keys)))
	for _, key := range keys {
		e.writeString(key)
		e.writeString(value[key])
	}
}

func (e *canonicalEncoder) writeBytesMap(value map[string][]byte) {
	if value == nil {
		_ = e.buffer.WriteByte(0)
		return
	}
	_ = e.buffer.WriteByte(1)
	keys := slices.Sorted(maps.Keys(value))
	_ = binary.Write(&e.buffer, binary.BigEndian, uint32(len(keys)))
	for _, key := range keys {
		e.writeString(key)
		e.writeBytes(value[key])
	}
}
