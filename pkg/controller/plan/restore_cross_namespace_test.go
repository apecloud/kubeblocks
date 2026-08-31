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

package plan

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestBuildPrepareDataRestorePreservesBackupNamespace(t *testing.T) {
	manager := NewRestoreManager(context.Background(), nil, &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "target", UID: "12345678"},
	}, nil, nil, 1, 0)
	comp := &component.SynthesizedComponent{
		Name: "mysql",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "source"},
		Status: dpv1alpha1.BackupStatus{BackupMethod: &dpv1alpha1.BackupMethod{
			TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}},
		}},
	}

	restore, err := manager.BuildPrepareDataRestore(comp, backup, nil)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() error = %v", err)
	}
	if restore.Spec.Backup.Namespace != backup.Namespace {
		t.Fatalf("backup namespace = %q, want %q", restore.Spec.Backup.Namespace, backup.Namespace)
	}
}
