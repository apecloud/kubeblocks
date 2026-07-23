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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const restoreRequestNamePrefix = "system-account-restore-"

const restoreRequestRequeueAfter = time.Second

// RestoreFailureRequiresReceiptCleanup identifies durable failures whose exact
// target receipt must be cleared idempotently before request cleanup completes.
func RestoreFailureRequiresReceiptCleanup(reason string) bool {
	return reason == TargetSemanticUnavailableReason ||
		reason == RequestDeletionRequestedReason ||
		reason == PostWriteCancellationReason
}

type TargetSemanticError struct {
	Reason string
	Cause  error
}

func (e *TargetSemanticError) Error() string {
	return fmt.Sprintf("%s: %v", e.Reason, e.Cause)
}

func (e *TargetSemanticError) Unwrap() error {
	return e.Cause
}

func NewTargetSemanticError(reason string, cause error) error {
	if reason != AccountUnavailableReason && reason != TargetSemanticUnavailableReason {
		reason = TargetSemanticUnavailableReason
	}
	return &TargetSemanticError{Reason: reason, Cause: cause}
}

type restoreRequestControllerIdentity struct {
	APIVersion string    `json:"apiVersion"`
	Kind       string    `json:"kind"`
	Name       string    `json:"name"`
	UID        types.UID `json:"uid"`
}

type restoreRequestRevisionPayload struct {
	Namespace  string                           `json:"namespace"`
	TargetName string                           `json:"targetName"`
	Type       corev1.SecretType                `json:"type"`
	Immutable  *bool                            `json:"immutable,omitempty"`
	Data       map[string][]byte                `json:"data"`
	Controller restoreRequestControllerIdentity `json:"controller"`
}

// RestoreRequestName returns the stable name of the replacement request for a
// system account Secret. There is exactly one active request per target name.
func RestoreRequestName(namespace, targetName string) string {
	digest := sha256.Sum256([]byte(namespace + "/" + targetName))
	return restoreRequestNamePrefix + hex.EncodeToString(digest[:8])
}

// SetRestoreRevision seals the request payload with a content-addressed
// revision. Apps copies this revision to the target as the durable completion
// marker observed by DataProtection. Labels and non-protocol annotations are
// intentionally outside the seal so admission-added metadata remains valid.
func SetRestoreRevision(request *corev1.Secret) error {
	revision, err := restoreRevision(request)
	if err != nil {
		return err
	}
	if request.Annotations == nil {
		request.Annotations = map[string]string{}
	}
	request.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey] = revision
	return nil
}

// ValidateRestoreRequest verifies the public metadata contract and the sealed
// payload before Apps acts on a request.
func ValidateRestoreRequest(request *corev1.Secret) error {
	targetName := request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey]
	if targetName == "" {
		return fmt.Errorf("system account restore request %s/%s has no target", request.Namespace, request.Name)
	}
	if request.Name != RestoreRequestName(request.Namespace, targetName) {
		return fmt.Errorf("system account restore request %s/%s has a non-canonical name", request.Namespace, request.Name)
	}
	if request.Labels[constant.SystemAccountRestoreRequestLabelKey] != "true" {
		return fmt.Errorf("system account restore request %s/%s has no request label", request.Namespace, request.Name)
	}
	if request.Annotations[constant.SystemAccountProvisionedAnnotationKey] != "true" {
		return fmt.Errorf("system account restore request %s/%s is not marked provisioned", request.Namespace, request.Name)
	}
	if _, ok := request.Data[constant.AccountNameForSecret]; !ok {
		return fmt.Errorf("system account restore request %s/%s has no account name", request.Namespace, request.Name)
	}
	if _, ok := request.Data[constant.AccountPasswdForSecret]; !ok {
		return fmt.Errorf("system account restore request %s/%s has no account password", request.Namespace, request.Name)
	}
	expected, err := restoreRevision(request)
	if err != nil {
		return err
	}
	if actual := request.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey]; actual != expected {
		return fmt.Errorf("system account restore request %s/%s has an invalid revision", request.Namespace, request.Name)
	}
	return nil
}

// RestoreConverged reports whether Apps has committed this exact request to
// the target. The persisted revision prevents an old request from being
// mistaken for the current restore after a same-name replacement race.
func RestoreConverged(target, request *corev1.Secret, ownedFinalizer string) bool {
	if target == nil || request == nil || !target.DeletionTimestamp.IsZero() {
		return false
	}
	if target.Namespace != request.Namespace ||
		target.Name != request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey] ||
		target.Type != request.Type ||
		!reflect.DeepEqual(target.Immutable, request.Immutable) ||
		!reflect.DeepEqual(target.Data, request.Data) ||
		!sameControllerIdentity(metav1.GetControllerOf(target), metav1.GetControllerOf(request)) ||
		!controllerutil.ContainsFinalizer(target, ownedFinalizer) {
		return false
	}
	if target.Labels[constant.SystemAccountRestoreRequestLabelKey] != "" ||
		target.Annotations[constant.SystemAccountRestoreTargetAnnotationKey] != "" {
		return false
	}
	for key, value := range requestTargetLabels(request) {
		if target.Labels[key] != value {
			return false
		}
	}
	for key, value := range requestTargetAnnotations(request) {
		if target.Annotations[key] != value {
			return false
		}
	}
	return true
}

func restoreRevision(request *corev1.Secret) (string, error) {
	targetName := request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey]
	if targetName == "" {
		return "", fmt.Errorf("system account restore request %s/%s has no target", request.Namespace, request.Name)
	}
	controller := metav1.GetControllerOf(request)
	if controller == nil {
		return "", fmt.Errorf("system account restore request %s/%s has no controller owner", request.Namespace, request.Name)
	}
	payload := restoreRequestRevisionPayload{
		Namespace:  request.Namespace,
		TargetName: targetName,
		Type:       request.Type,
		Immutable:  request.Immutable,
		Data:       request.Data,
		Controller: restoreRequestControllerIdentity{
			APIVersion: controller.APIVersion,
			Kind:       controller.Kind,
			Name:       controller.Name,
			UID:        controller.UID,
		},
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("serialize system account restore request %s/%s: %w", request.Namespace, request.Name, err)
	}
	digest := sha256.Sum256(serialized)
	return hex.EncodeToString(digest[:]), nil
}

// ReconcileRestoreRequests lets the Apps owner converge account Secret restore
// requests. The caller owns all target mutations, finalizer handling, and
// delete/recreate operations; the request writer never performs them.
//
// A request is discovered by its immutable restore-intent envelope, then
// strong-read and matched to the target owner's sealed identity before any
// mutable protocol metadata is trusted. Apps keeps the request until it has
// copied the requested credentials and exact commit receipt to an owned target.
// This makes the target receipt the commit point and the request the durable
// retry state across delete/create races.
func ReconcileRestoreRequests(ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	operationReader client.Reader,
	owner client.Object,
	ownedFinalizer string,
	targetBuilders ...func(CredentialIntent) (*corev1.Secret, error)) (bool, error) {
	var targetBuilder func(CredentialIntent) (*corev1.Secret, error)
	if len(targetBuilders) > 0 {
		targetBuilder = targetBuilders[0]
	}
	if operationReader == nil {
		return false, fmt.Errorf("system account restore authority reader is not configured")
	}
	requests := &corev1.SecretList{}
	if err := graphCli.List(ctx, requests,
		client.InNamespace(owner.GetNamespace())); err != nil {
		return false, err
	}
	slices.SortFunc(requests.Items, func(a, b corev1.Secret) int {
		return cmp.Compare(a.Name, b.Name)
	})
	handled := false
	seenTargets := map[string]string{}
	for i := range requests.Items {
		request := &requests.Items[i]
		if !HasRestoreIntentEnvelope(request) {
			continue
		}
		liveRequest := &corev1.Secret{}
		if err := operationReader.Get(ctx, client.ObjectKeyFromObject(request), liveRequest); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return true, err
		}
		if liveRequest.UID != request.UID {
			continue
		}
		request = liveRequest
		envelope, err := DecodeRestoreIntentEnvelope(request)
		if err != nil || !sameObjectIdentity(envelope.Target.Owner, owner) {
			continue
		}
		handled = true
		intent, err := ValidateRestoreRequestV2(request)
		if err != nil {
			continue
		}
		targetDigest := request.Annotations[LogicalTargetDigestAnnotationKey]
		if previous, ok := seenTargets[targetDigest]; ok {
			return true, fmt.Errorf("multiple system account restore requests %s and %s target logical slot %s",
				previous, request.Name, targetDigest)
		}
		seenTargets[targetDigest] = request.Name
		if err := reconcileRestoreRequestV2(ctx, graphCli, dag, operationReader, owner, ownedFinalizer,
			request, intent, targetBuilder); err != nil {
			return true, err
		}
	}
	if handled {
		return true, intctrlutil.NewRequeueError(restoreRequestRequeueAfter,
			fmt.Sprintf("waiting for system account restore requests owned by %s/%s", owner.GetNamespace(), owner.GetName()))
	}
	return handled, nil
}

func reconcileRestoreRequestV2(
	ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	operationReader client.Reader,
	owner client.Object,
	ownedFinalizer string,
	request *corev1.Secret,
	intent CredentialIntent,
	targetBuilder func(CredentialIntent) (*corev1.Secret, error),
) error {
	phase := RestoreRequestPhase(request.Annotations[RestoreRequestPhaseAnnotationKey])
	switch phase {
	case RestoreRequestPhasePending:
		if operationReader == nil {
			return fmt.Errorf("system account restore request %s/%s has no operation authority reader",
				request.Namespace, request.Name)
		}
		operationState, err := ReadRestoreOperationState(ctx, operationReader, intent.Operation)
		if err != nil {
			return err
		}
		switch operationState {
		case RestoreOperationSucceeded, RestoreOperationFailed:
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, OperationTerminalReason, nil)
		case RestoreOperationGone, RestoreOperationTerminating:
			// The lifecycle controller projects the stable root reason and
			// releases the protocol finalizer without touching the target.
			return nil
		}
		if !request.DeletionTimestamp.IsZero() {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, RequestDeletionRequestedReason, nil)
		}
		ownerLive, err := ReadRestoreTargetOwnerLive(ctx, operationReader, intent.Target.Owner)
		if err != nil {
			return err
		}
		if !ownerLive {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, TargetOwnerUnavailableReason, nil)
		}
		if targetBuilder == nil {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, TargetSemanticUnavailableReason, nil)
		}
		if _, err := targetBuilder(intent); err != nil {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, targetSemanticFailureReason(err), nil)
		}
		return updateRestoreRequestState(graphCli, dag, request, RestoreRequestPhaseClaimed, "", nil)
	case RestoreRequestPhaseClaimed, RestoreRequestPhaseCommitted:
		return reconcileClaimedRestoreRequestV2(ctx, graphCli, dag, operationReader, owner, ownedFinalizer,
			request, intent, targetBuilder)
	case RestoreRequestPhaseFailed:
		if RestoreFailureRequiresReceiptCleanup(
			request.Annotations[RestoreRequestReasonAnnotationKey]) {
			target, exact, err := findLinkedTarget(
				ctx, operationReader, request, intent, ownedFinalizer)
			if err != nil {
				return err
			}
			if exact {
				updated := target.DeepCopy()
				ClearTargetRestoreReceipt(updated)
				graphCli.Update(dag, target, updated)
			}
		}
		return nil
	default:
		return fmt.Errorf("system account restore request %s/%s has invalid phase %q",
			request.Namespace, request.Name, phase)
	}
}

func reconcileClaimedRestoreRequestV2(
	ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	operationReader client.Reader,
	owner client.Object,
	ownedFinalizer string,
	request *corev1.Secret,
	intent CredentialIntent,
	targetBuilder func(CredentialIntent) (*corev1.Secret, error),
) error {
	if operationReader == nil {
		return fmt.Errorf("system account restore request %s/%s has no operation authority reader",
			request.Namespace, request.Name)
	}
	operationState, err := ReadRestoreOperationState(ctx, operationReader, intent.Operation)
	if err != nil {
		return err
	}
	if operationState != RestoreOperationActive {
		_, exact, err := findLinkedTarget(ctx, operationReader, request, intent, ownedFinalizer)
		if err != nil {
			return err
		}
		if exact {
			// The target already contains this exact committed write. The
			// lifecycle controller repairs or commits the request receipt
			// after its own operation-state recheck.
			return nil
		}
		if operationState == RestoreOperationSucceeded || operationState == RestoreOperationFailed {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, OperationTerminalReason, nil)
		}
		// Gone and Terminating roots are projected by the independent
		// lifecycle controller. Do not create or update account data.
		return nil
	}
	if !request.DeletionTimestamp.IsZero() {
		return updateRestoreRequestState(graphCli, dag, request,
			RestoreRequestPhaseFailed, RequestDeletionRequestedReason, nil)
	}
	ownerLive, err := ReadRestoreTargetOwnerLive(ctx, operationReader, intent.Target.Owner)
	if err != nil {
		return err
	}
	if !ownerLive {
		// The lifecycle controller distinguishes a pre-write owner loss from
		// a post-write cancellation by checking the exact live target receipt.
		return nil
	}
	if targetBuilder == nil {
		return updateRestoreRequestState(graphCli, dag, request,
			RestoreRequestPhaseFailed, TargetSemanticUnavailableReason, nil)
	}
	desired, err := targetBuilder(intent)
	if err != nil {
		reason := targetSemanticFailureReason(err)
		if reason == AccountUnavailableReason {
			_, exact, findErr := findLinkedTarget(
				ctx, operationReader, request, intent, ownedFinalizer)
			if findErr != nil {
				return findErr
			}
			phase := RestoreRequestPhase(request.Annotations[RestoreRequestPhaseAnnotationKey])
			if exact {
				if phase == RestoreRequestPhaseCommitted {
					return nil
				}
				// The independent lifecycle controller owns the final
				// post-write operation/owner check and Committed transition.
				return nil
			}
			if phase == RestoreRequestPhaseCommitted {
				reason = CredentialContinuityLostReason
			}
		}
		return updateRestoreRequestState(graphCli, dag, request,
			RestoreRequestPhaseFailed, reason, nil)
	}
	if desired.Namespace != intent.Target.Namespace || desired.Name == "" {
		return updateRestoreRequestState(graphCli, dag, request,
			RestoreRequestPhaseFailed, TargetSemanticUnavailableReason,
			nil)
	}
	if !sameControllerIdentity(metav1.GetControllerOf(desired), objectOwnerReference(intent.Target.Owner)) ||
		!controllerutil.ContainsFinalizer(desired, ownedFinalizer) {
		return updateRestoreRequestState(graphCli, dag, request,
			RestoreRequestPhaseFailed, TargetSemanticUnavailableReason,
			nil)
	}
	applyTargetReceipt(desired, request)
	revision, err := TargetCommitRevision(desired, ownedFinalizer)
	if err != nil {
		return err
	}
	desired.Annotations[TargetCommitRevisionAnnotationKey] = revision

	target := &corev1.Secret{}
	key := client.ObjectKeyFromObject(desired)
	if err := operationReader.Get(ctx, key, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		graphCli.Create(dag, desired)
		return nil
	}
	if !sameControllerIdentity(metav1.GetControllerOf(target),
		objectOwnerReference(intent.Target.Owner)) {
		if len(target.OwnerReferences) > 0 || !target.DeletionTimestamp.IsZero() {
			return updateRestoreRequestState(graphCli, dag, request,
				RestoreRequestPhaseFailed, TargetOwnerUnavailableReason,
				nil)
		}
		adopted := target.DeepCopy()
		if err := intctrlutil.SetOwnership(owner, adopted, model.GetScheme(), ownedFinalizer); err != nil {
			return err
		}
		graphCli.Update(dag, target, adopted)
		return nil
	}
	if !target.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(target, ownedFinalizer) {
			updated := target.DeepCopy()
			controllerutil.RemoveFinalizer(updated, ownedFinalizer)
			graphCli.Update(dag, target, updated)
		}
		return nil
	}

	preserveUnownedTargetMetadata(target, desired)
	applyTargetReceipt(desired, request)
	desired.ResourceVersion = target.ResourceVersion
	desired.UID = target.UID
	desired.CreationTimestamp = target.CreationTimestamp
	desired.ManagedFields = target.ManagedFields
	revision, err = TargetCommitRevision(desired, ownedFinalizer)
	if err != nil {
		return err
	}
	desired.Annotations[TargetCommitRevisionAnnotationKey] = revision
	if !reflect.DeepEqual(target, desired) {
		if target.Immutable != nil && *target.Immutable &&
			(!reflect.DeepEqual(target.Data, desired.Data) ||
				!reflect.DeepEqual(target.Immutable, desired.Immutable) ||
				target.Type != desired.Type) {
			return graphCli.Delete(dag, target, model.WithDeleteUID(target.UID))
		}
		graphCli.Update(dag, target, desired)
		return nil
	}
	// Target and receipt are exact. The lifecycle controller re-reads the
	// operation and owner before it commits the request.
	return nil
}

func preserveUnownedTargetMetadata(current, desired *corev1.Secret) {
	labels := maps.Clone(current.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, desired.Labels)
	desired.Labels = labels

	annotations := maps.Clone(current.Annotations)
	if annotations == nil {
		annotations = map[string]string{}
	}
	maps.Copy(annotations, desired.Annotations)
	desired.Annotations = annotations

	finalizers := slices.Clone(current.Finalizers)
	for _, finalizer := range desired.Finalizers {
		if !slices.Contains(finalizers, finalizer) {
			finalizers = append(finalizers, finalizer)
		}
	}
	desired.Finalizers = finalizers

	ownerReferences := slices.Clone(desired.OwnerReferences)
	for _, ownerReference := range current.OwnerReferences {
		if slices.ContainsFunc(ownerReferences, func(existing metav1.OwnerReference) bool {
			return sameOwnerReferenceIdentity(existing, ownerReference)
		}) {
			continue
		}
		ownerReferences = append(ownerReferences, ownerReference)
	}
	desired.OwnerReferences = ownerReferences
}

func findLinkedTarget(
	ctx context.Context,
	reader client.Reader,
	request *corev1.Secret,
	intent CredentialIntent,
	ownedFinalizer string,
) (*corev1.Secret, bool, error) {
	if reader == nil {
		return nil, false, fmt.Errorf("system account restore target authority reader is not configured")
	}
	if name := request.Annotations[TargetSecretNameAnnotationKey]; name != "" {
		target := &corev1.Secret{}
		if err := reader.Get(ctx, client.ObjectKey{
			Namespace: request.Namespace,
			Name:      name,
		}, target); err != nil {
			if !apierrors.IsNotFound(err) {
				return nil, false, err
			}
		} else if TargetReceiptExactV2(target, request, ownedFinalizer) {
			return target, true, nil
		}
	}
	targets := &corev1.SecretList{}
	if err := reader.List(ctx, targets, client.InNamespace(intent.Target.Namespace)); err != nil {
		return nil, false, err
	}
	var exactTarget *corev1.Secret
	for i := range targets.Items {
		target := &targets.Items[i]
		if target.Annotations[RestoreRequestNameAnnotationKey] == request.Name &&
			target.Annotations[RestoreRequestUIDAnnotationKey] == string(request.UID) &&
			TargetReceiptExactV2(target, request, ownedFinalizer) {
			if exactTarget != nil {
				return nil, false, fmt.Errorf(
					"multiple exact targets %s/%s and %s/%s link restore request %s/%s",
					exactTarget.Namespace, exactTarget.Name, target.Namespace, target.Name,
					request.Namespace, request.Name)
			}
			exactTarget = target
		}
	}
	if exactTarget != nil {
		return exactTarget, true, nil
	}
	return nil, false, nil
}

func targetSemanticFailureReason(err error) string {
	var semanticErr *TargetSemanticError
	if errors.As(err, &semanticErr) {
		return semanticErr.Reason
	}
	return TargetSemanticUnavailableReason
}

func updateRestoreRequestState(
	graphCli model.GraphClient,
	dag *graph.DAG,
	request *corev1.Secret,
	phase RestoreRequestPhase,
	reason string,
	annotations map[string]string,
) error {
	var receipt *RestoreCommitReceipt
	if phase == RestoreRequestPhaseCommitted {
		receipt = &RestoreCommitReceipt{
			TargetName:     annotations[TargetSecretNameAnnotationKey],
			TargetUID:      types.UID(annotations[TargetSecretUIDAnnotationKey]),
			CommitRevision: annotations[TargetCommitRevisionAnnotationKey],
		}
	}
	updated, err := TransitionRestoreRequestV2(request, phase, reason, receipt, false)
	if err != nil {
		return err
	}
	graphCli.Update(dag, request, updated)
	return nil
}

func applyTargetReceipt(target, request *corev1.Secret) {
	if target.Annotations == nil {
		target.Annotations = map[string]string{}
	}
	target.Annotations[constant.SystemAccountProvisionedAnnotationKey] = "true"
	target.Annotations[RestoreProtocolAnnotationKey] = RestoreProtocolV2
	target.Annotations[RestoreOperationDigestAnnotationKey] =
		request.Annotations[RestoreOperationDigestAnnotationKey]
	target.Annotations[CredentialIntentRevisionAnnotationKey] =
		request.Annotations[CredentialIntentRevisionAnnotationKey]
	target.Annotations[RestoreRequestNameAnnotationKey] = request.Name
	target.Annotations[RestoreRequestUIDAnnotationKey] = string(request.UID)
	delete(target.Annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	delete(target.Labels, constant.SystemAccountRestoreRequestLabelKey)
}

func objectOwnerReference(identity ObjectIdentity) *metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return &metav1.OwnerReference{
		APIVersion:         identity.APIVersion,
		Kind:               identity.Kind,
		Name:               identity.Name,
		UID:                identity.UID,
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

func sameObjectIdentity(identity ObjectIdentity, object client.Object) bool {
	if object == nil {
		return false
	}
	gvk, err := apiutil.GVKForObject(object, model.GetScheme())
	if err != nil {
		return false
	}
	return identity.APIVersion == gvk.GroupVersion().String() &&
		identity.Kind == gvk.Kind &&
		identity.Namespace == object.GetNamespace() &&
		identity.Name == object.GetName() &&
		identity.UID == object.GetUID()
}

func requestTargetLabels(request *corev1.Secret) map[string]string {
	labels := maps.Clone(request.Labels)
	delete(labels, constant.SystemAccountRestoreRequestLabelKey)
	return labels
}

func requestTargetAnnotations(request *corev1.Secret) map[string]string {
	annotations := maps.Clone(request.Annotations)
	delete(annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	return annotations
}

func sameControllerIdentity(a, b *metav1.OwnerReference) bool {
	return a != nil && b != nil &&
		sameOwnerReferenceIdentity(*a, *b)
}

func sameOwnerReferenceIdentity(a, b metav1.OwnerReference) bool {
	return a.APIVersion == b.APIVersion &&
		a.Kind == b.Kind &&
		a.Name == b.Name &&
		a.UID == b.UID
}
