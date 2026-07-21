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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

const restoreRequestNamePrefix = "system-account-restore-"

const restoreRequestRequeueAfter = time.Second

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
// A request is a Secret selected by SystemAccountRestoreRequestLabelKey,
// controlled by the target Component or Cluster, and sealed by
// SystemAccountRestoreRevisionAnnotationKey. Apps keeps the request until it
// has copied that revision and the requested credentials to an owned target.
// This makes the target revision the commit point and the request the durable
// retry state across delete/create races.
func ReconcileRestoreRequests(ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	owner client.Object,
	ownedFinalizer string) (bool, error) {
	requests := &corev1.SecretList{}
	if err := graphCli.List(ctx, requests,
		client.InNamespace(owner.GetNamespace()),
		client.MatchingLabels{constant.SystemAccountRestoreRequestLabelKey: "true"}); err != nil {
		return false, err
	}
	slices.SortFunc(requests.Items, func(a, b corev1.Secret) int {
		return cmp.Compare(a.Name, b.Name)
	})
	handled := false
	seenTargets := map[string]string{}
	for i := range requests.Items {
		request := &requests.Items[i]
		targetName := request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey]
		if !metav1.IsControlledBy(request, owner) {
			continue
		}
		handled = true
		if err := ValidateRestoreRequest(request); err != nil {
			return true, err
		}
		if previous, ok := seenTargets[targetName]; ok {
			return true, fmt.Errorf("multiple system account restore requests %s and %s target %s/%s",
				previous, request.Name, request.Namespace, targetName)
		}
		seenTargets[targetName] = request.Name
		if err := reconcileRestoreRequest(ctx, graphCli, dag, owner, ownedFinalizer, request, targetName); err != nil {
			return true, err
		}
	}
	if handled {
		return true, intctrlutil.NewRequeueError(restoreRequestRequeueAfter,
			fmt.Sprintf("waiting for system account restore requests owned by %s/%s", owner.GetNamespace(), owner.GetName()))
	}
	return handled, nil
}

func reconcileRestoreRequest(ctx context.Context,
	graphCli model.GraphClient,
	dag *graph.DAG,
	owner client.Object,
	ownedFinalizer string,
	request *corev1.Secret,
	targetName string) error {
	if !request.DeletionTimestamp.IsZero() {
		return nil
	}

	target := &corev1.Secret{}
	key := client.ObjectKey{Namespace: request.Namespace, Name: targetName}
	if err := graphCli.Get(ctx, key, target); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		desired, err := desiredTarget(request, owner, ownedFinalizer)
		if err != nil {
			return err
		}
		graphCli.Create(dag, desired)
		return nil
	}

	expectedOwner := metav1.GetControllerOf(request)
	if expectedOwner == nil {
		return fmt.Errorf("system account restore request %s/%s has no controller owner", request.Namespace, request.Name)
	}
	if !hasOwnerReference(target, expectedOwner) {
		if len(target.OwnerReferences) > 0 || !target.DeletionTimestamp.IsZero() {
			return fmt.Errorf("system account restore target %s is not owned by %s %s",
				key, expectedOwner.Kind, expectedOwner.Name)
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

	desired := target.DeepCopy()
	desired.Type = request.Type
	desired.Immutable = request.Immutable
	desired.Data = maps.Clone(request.Data)
	mergeStringMap(requestTargetLabels(request), &desired.Labels)
	delete(desired.Labels, constant.SystemAccountRestoreRequestLabelKey)
	mergeStringMap(requestTargetAnnotations(request), &desired.Annotations)
	delete(desired.Annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	if err := intctrlutil.SetOwnership(owner, desired, model.GetScheme(), ownedFinalizer); err != nil {
		return err
	}
	if reflect.DeepEqual(target, desired) {
		if controllerutil.ContainsFinalizer(request, ownedFinalizer) {
			updated := request.DeepCopy()
			controllerutil.RemoveFinalizer(updated, ownedFinalizer)
			graphCli.Update(dag, request, updated)
		} else {
			graphCli.Delete(dag, request)
		}
		return nil
	}
	if target.Immutable != nil && *target.Immutable &&
		(!reflect.DeepEqual(target.Data, desired.Data) ||
			!reflect.DeepEqual(target.Immutable, desired.Immutable) ||
			target.Type != desired.Type) {
		graphCli.Delete(dag, target)
		return nil
	}
	graphCli.Update(dag, target, desired)
	return nil
}

func desiredTarget(request *corev1.Secret, owner client.Object, ownedFinalizer string) (*corev1.Secret, error) {
	targetName := request.Annotations[constant.SystemAccountRestoreTargetAnnotationKey]
	if targetName == "" {
		return nil, fmt.Errorf("system account restore request %s/%s has no target", request.Namespace, request.Name)
	}
	annotations := maps.Clone(request.Annotations)
	delete(annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   request.Namespace,
			Name:        targetName,
			Labels:      requestTargetLabels(request),
			Annotations: annotations,
		},
		Immutable: request.Immutable,
		Type:      request.Type,
		Data:      maps.Clone(request.Data),
	}
	if err := intctrlutil.SetOwnership(owner, target, model.GetScheme(), ownedFinalizer); err != nil {
		return nil, err
	}
	return target, nil
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
		a.APIVersion == b.APIVersion &&
		a.Kind == b.Kind &&
		a.Name == b.Name &&
		a.UID == b.UID
}

func hasOwnerReference(object client.Object, expected *metav1.OwnerReference) bool {
	return expected != nil && slices.ContainsFunc(object.GetOwnerReferences(), func(actual metav1.OwnerReference) bool {
		return actual.APIVersion == expected.APIVersion &&
			actual.Kind == expected.Kind &&
			actual.Name == expected.Name &&
			actual.UID == expected.UID
	})
}

func mergeStringMap(from map[string]string, to *map[string]string) {
	if len(from) == 0 {
		return
	}
	if *to == nil {
		*to = map[string]string{}
	}
	for key, value := range from {
		(*to)[key] = value
	}
}
