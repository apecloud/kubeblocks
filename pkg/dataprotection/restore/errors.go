/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
	"errors"
	"fmt"
)

// StorageClassNotFoundError signals that a referenced StorageClass cannot be
// resolved at the time the data-protection restore path attempts to read it.
//
// This typed error is returned by helpers in the data-protection restore path
// (e.g. VolumePopulator.waitForPVCSelectedNode) when a StorageClass GET fails
// with apierrors.NotFound. Callers, for example ClusterRestoreReconciler,
// use IsStorageClassNotFoundError to translate this case into a user-facing
// condition (WaitingForStorageClass) with a bounded retry window before
// escalating to Failed/StorageClassMissing. Other GET errors continue to
// propagate as raw errors and rely on controller-runtime default retry.
type StorageClassNotFoundError struct {
	// Name is the StorageClass that the helper attempted to GET.
	Name string
}

// Error implements the error interface. Format intentionally mirrors the
// kube-apiserver NotFound message so existing log output stays human-readable.
func (e *StorageClassNotFoundError) Error() string {
	return fmt.Sprintf("StorageClass %q not found", e.Name)
}

// IsStorageClassNotFoundError reports whether err (or any error it wraps)
// is a *StorageClassNotFoundError, returning the embedded sentinel so the
// caller can read Name. Returns (nil, false) when err does not carry one.
func IsStorageClassNotFoundError(err error) (*StorageClassNotFoundError, bool) {
	var scErr *StorageClassNotFoundError
	if errors.As(err, &scErr) {
		return scErr, true
	}
	return nil, false
}
