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
	"testing"
)

// Minimal red-tests pinning the *StorageClassNotFoundError contract used by
// VolumePopulator.waitForPVCSelectedNode to signal a missing StorageClass and
// by ClusterRestoreReconciler to translate that case into a user-facing
// WaitingForStorageClass condition. Caller-side reconcile behaviour, bounded
// timeout, and Warning event coverage live with the ClusterRestore tests; this
// file only locks the sentinel surface.

func TestStorageClassNotFoundError_Error(t *testing.T) {
	err := &StorageClassNotFoundError{Name: "apelocal-hostpath-data"}
	got := err.Error()
	want := `StorageClass "apelocal-hostpath-data" not found`
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestIsStorageClassNotFoundError(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if scErr, ok := IsStorageClassNotFoundError(nil); ok || scErr != nil {
			t.Fatalf("nil err should not match, got (%v, %v)", scErr, ok)
		}
	})

	t.Run("plain other error", func(t *testing.T) {
		if scErr, ok := IsStorageClassNotFoundError(errors.New("boom")); ok || scErr != nil {
			t.Fatalf("plain error should not match, got (%v, %v)", scErr, ok)
		}
	})

	t.Run("direct sentinel", func(t *testing.T) {
		direct := &StorageClassNotFoundError{Name: "sc-x"}
		scErr, ok := IsStorageClassNotFoundError(direct)
		if !ok || scErr == nil {
			t.Fatalf("direct sentinel should match, got (%v, %v)", scErr, ok)
		}
		if scErr.Name != "sc-x" {
			t.Fatalf("scErr.Name = %q, want %q", scErr.Name, "sc-x")
		}
	})

	t.Run("wrapped sentinel via fmt.Errorf %w", func(t *testing.T) {
		wrapped := fmt.Errorf("populate failed: %w", &StorageClassNotFoundError{Name: "sc-y"})
		scErr, ok := IsStorageClassNotFoundError(wrapped)
		if !ok || scErr == nil {
			t.Fatalf("wrapped sentinel should match via errors.As, got (%v, %v)", scErr, ok)
		}
		if scErr.Name != "sc-y" {
			t.Fatalf("scErr.Name = %q, want %q", scErr.Name, "sc-y")
		}
	})
}
