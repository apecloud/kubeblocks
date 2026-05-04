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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
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

func TestRestoreOptionsRoundTrip(t *testing.T) {
	options := RestoreOptions{
		BackupNamespace:                   "source-ns",
		RestoreTime:                       "2026-05-04T08:00:00Z",
		VolumeSource:                      "data",
		MountPath:                         "/data",
		SourceTargetName:                  "target-0",
		SourceTargetPodName:               "target-0-pod-0",
		VolumeRestorePolicy:               dpv1alpha1.VolumeClaimRestorePolicySerial,
		DeferPostReadyUntilClusterRunning: true,
		Env: []corev1.EnvVar{{
			Name:  "RESTORE_MODE",
			Value: "pitr",
		}},
		Parameters: []dpv1alpha1.ParameterPair{{
			Name:  "parallelism",
			Value: "4",
		}},
	}

	annotations, err := SetRestoreOptions(nil, options)
	if err != nil {
		t.Fatalf("SetRestoreOptions returned error: %v", err)
	}
	if annotations[dptypes.RestoreOptionsAnnotationKey] == "" {
		t.Fatalf("restore options annotation %q is empty", dptypes.RestoreOptionsAnnotationKey)
	}
	parsed, err := ParseRestoreOptions(annotations)
	if err != nil {
		t.Fatalf("ParseRestoreOptions returned error: %v", err)
	}
	if parsed.BackupNamespace != options.BackupNamespace ||
		parsed.RestoreTime != options.RestoreTime ||
		parsed.VolumeSource != options.VolumeSource ||
		parsed.MountPath != options.MountPath ||
		parsed.SourceTargetName != options.SourceTargetName ||
		parsed.SourceTargetPodName != options.SourceTargetPodName ||
		parsed.VolumeRestorePolicy != options.VolumeRestorePolicy ||
		parsed.DeferPostReadyUntilClusterRunning != options.DeferPostReadyUntilClusterRunning {
		t.Fatalf("parsed options = %#v, want %#v", parsed, options)
	}
	if len(parsed.Env) != 1 || parsed.Env[0].Name != "RESTORE_MODE" || parsed.Env[0].Value != "pitr" {
		t.Fatalf("parsed env = %#v", parsed.Env)
	}
	if len(parsed.Parameters) != 1 || parsed.Parameters[0].Name != "parallelism" || parsed.Parameters[0].Value != "4" {
		t.Fatalf("parsed parameters = %#v", parsed.Parameters)
	}
	if parsed.VolumeConfig().VolumeSource != "data" || parsed.VolumeConfig().MountPath != "/data" {
		t.Fatalf("volume config = %#v", parsed.VolumeConfig())
	}
}

func TestParseRestoreOptionsDefaults(t *testing.T) {
	options, err := ParseRestoreOptions(nil)
	if err != nil {
		t.Fatalf("ParseRestoreOptions returned error: %v", err)
	}
	if options.VolumeRestorePolicy != dpv1alpha1.VolumeClaimRestorePolicyParallel {
		t.Fatalf("volumeRestorePolicy = %q, want Parallel", options.VolumeRestorePolicy)
	}

	annotations, err := SetRestoreOptions(map[string]string{"keep": "me"}, RestoreOptions{VolumeSource: "data"})
	if err != nil {
		t.Fatalf("SetRestoreOptions returned error: %v", err)
	}
	if annotations["keep"] != "me" {
		t.Fatalf("existing annotation not preserved")
	}
	parsed, err := ParseRestoreOptions(annotations)
	if err != nil {
		t.Fatalf("ParseRestoreOptions returned error: %v", err)
	}
	if parsed.VolumeRestorePolicy != dpv1alpha1.VolumeClaimRestorePolicyParallel {
		t.Fatalf("volumeRestorePolicy = %q, want Parallel", parsed.VolumeRestorePolicy)
	}
}

func TestRestoreOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		annotation  string
		errContains string
	}{
		{
			name:        "invalid restore time",
			annotation:  `{"restoreTime":"bad-time"}`,
			errContains: "restoreTime must be RFC3339",
		},
		{
			name:        "invalid volume restore policy",
			annotation:  `{"volumeRestorePolicy":"Fast"}`,
			errContains: "unsupported volumeRestorePolicy",
		},
		{
			name:        "unknown field",
			annotation:  `{"unknown":true}`,
			errContains: "unknown field",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRestoreOptions(map[string]string{
				dptypes.RestoreOptionsAnnotationKey: tt.annotation,
			})
			if err == nil {
				t.Fatalf("expected error")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.errContains)
			}
		})
	}
}
