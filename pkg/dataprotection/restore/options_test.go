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
	"testing"

	corev1 "k8s.io/api/core/v1"

	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestBackupDataSourceRef(t *testing.T) {
	ref := BackupDataSourceRef("backup")
	if ref == nil {
		t.Fatal("expected dataSourceRef")
	}
	if ref.APIGroup == nil || *ref.APIGroup != dptypes.DataprotectionAPIGroup {
		t.Fatalf("apiGroup = %v, want %q", ref.APIGroup, dptypes.DataprotectionAPIGroup)
	}
	if ref.Kind != dptypes.BackupKind {
		t.Fatalf("kind = %q, want %q", ref.Kind, dptypes.BackupKind)
	}
	if ref.Name != "backup" {
		t.Fatalf("name = %q, want backup", ref.Name)
	}
	if !IsBackupDataSourceRef(ref) {
		t.Fatal("expected DP Backup dataSourceRef to match")
	}
	if IsBackupDataSourceRef(&corev1.TypedObjectReference{Kind: dptypes.RestoreKind, Name: "restore", APIGroup: ref.APIGroup}) {
		t.Fatal("Restore dataSourceRef should not match Backup API")
	}
}
