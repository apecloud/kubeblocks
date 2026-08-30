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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func TestRestoreManagerBuildPrepareDataRestoreEnv(t *testing.T) {
	manager := &RestoreManager{
		Cluster: &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "cluster", Namespace: "default", UID: types.UID("12345678-rest-of-uid"),
		}},
		namespace: "default",
		replicas:  1,
	}
	restoreEnv := []corev1.EnvVar{{Name: "RESTORE_ENV", Value: "from-plan-intent"}}
	manager.SetRestoreEnv(restoreEnv)

	restore, err := manager.BuildPrepareDataRestore(&component.SynthesizedComponent{
		Name: "mysql",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}, &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup"},
		Status: dpv1alpha1.BackupStatus{BackupMethod: &dpv1alpha1.BackupMethod{
			Name:          "snapshot",
			TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() error = %v", err)
	}
	if restore == nil {
		t.Fatal("restore is nil")
	}
	if !reflect.DeepEqual(restore.Spec.Env, restoreEnv) {
		t.Fatalf("restore env = %#v, want %#v", restore.Spec.Env, restoreEnv)
	}
}
