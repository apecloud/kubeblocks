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

package restore

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func internalPostReadyRestoreNameCandidate(name string) bool {
	const (
		prefix = "restore-"
		suffix = "-post-ready"
	)
	return strings.HasPrefix(name, prefix) &&
		strings.HasSuffix(name, suffix) &&
		len(name) > len(prefix)+len(suffix)
}

// InternalPostReadyRestoreComponentOwner validates the durable identity of an
// internal postReady Restore. The boolean is false only for an external Restore
// that carries neither the reserved name shape nor the internal marker.
func InternalPostReadyRestoreComponentOwner(
	restore *dpv1alpha1.Restore,
) (*metav1.OwnerReference, bool, error) {
	if restore == nil {
		return nil, false, nil
	}
	marker, markerExists := restore.Labels[DataProtectionInternalPostReadyLabelKey]
	if !markerExists && !internalPostReadyRestoreNameCandidate(restore.Name) {
		return nil, false, nil
	}
	identityError := func(detail string) (*metav1.OwnerReference, bool, error) {
		return nil, true, intctrlutil.NewFatalError(fmt.Sprintf(
			"internal postReady restore %s/%s has invalid identity: %s",
			restore.Namespace, restore.Name, detail))
	}
	if marker != DataProtectionInternalPostReadyLabelValue {
		return identityError(fmt.Sprintf(
			"label %s must be %q",
			DataProtectionInternalPostReadyLabelKey,
			DataProtectionInternalPostReadyLabelValue))
	}
	if len(restore.OwnerReferences) != 1 {
		return identityError(fmt.Sprintf(
			"expected exactly one Component controller owner, got %d",
			len(restore.OwnerReferences)))
	}
	ref := &restore.OwnerReferences[0]
	if ref.APIVersion != appsv1.APIVersion {
		return identityError(fmt.Sprintf(
			"owner apiVersion must be %q, got %q", appsv1.APIVersion, ref.APIVersion))
	}
	if ref.Kind != appsv1.ComponentKind {
		return identityError(fmt.Sprintf(
			"owner kind must be %q, got %q", appsv1.ComponentKind, ref.Kind))
	}
	if ref.Controller == nil || !*ref.Controller {
		return identityError("Component owner must be the controller")
	}
	if ref.Name == "" {
		return identityError("Component owner name is empty")
	}
	if ref.UID == "" {
		return identityError("Component owner UID is empty")
	}
	if expectedName := PostReadyRestoreName(ref.UID); restore.Name != expectedName {
		return identityError(fmt.Sprintf(
			"name must be %q for Component UID %s", expectedName, ref.UID))
	}
	return ref, true, nil
}

// ValidateInternalPostReadyRestoreComponent validates the Restore identity
// against the Component expected by the VolumePopulator.
func ValidateInternalPostReadyRestoreComponent(
	restore *dpv1alpha1.Restore,
	component *appsv1.Component,
) error {
	if restore == nil {
		return intctrlutil.NewFatalError("internal postReady Restore is nil")
	}
	ref, internal, err := InternalPostReadyRestoreComponentOwner(restore)
	if err != nil {
		return err
	}
	if !internal {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"postReady restore %s/%s is missing its internal identity",
			restore.Namespace, restore.Name))
	}
	if component == nil {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"internal postReady restore %s/%s has no expected Component",
			restore.Namespace, restore.Name))
	}
	if restore.Namespace != component.Namespace ||
		ref.Name != component.Name ||
		ref.UID != component.UID {
		return intctrlutil.NewFatalError(fmt.Sprintf(
			"internal postReady restore %s/%s owner does not match Component %s/%s UID %s",
			restore.Namespace, restore.Name,
			component.Namespace, component.Name, component.UID))
	}
	return nil
}
