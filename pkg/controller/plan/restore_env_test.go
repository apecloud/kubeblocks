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
	"github.com/apecloud/kubeblocks/pkg/constant"
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

func TestRestoreManagerBuildPrepareDataRestoreSourceTargetOverride(t *testing.T) {
	manager := &RestoreManager{
		Cluster: &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name: "cluster", Namespace: "default", UID: types.UID("12345678-rest-of-uid"),
		}},
		namespace:        "default",
		replicas:         1,
		SourceTargetName: "target-b",
	}
	comp := &component.SynthesizedComponent{
		Name:        "mysql",
		Annotations: map[string]string{constant.BackupSourceTargetAnnotationKey: "target-a"},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup"},
		Status: dpv1alpha1.BackupStatus{
			Targets: []dpv1alpha1.BackupStatusTarget{
				{BackupTarget: dpv1alpha1.BackupTarget{Name: "target-a", PodSelector: &dpv1alpha1.PodSelector{}}},
				{BackupTarget: dpv1alpha1.BackupTarget{Name: "target-b", PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll}}},
			},
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name:          "snapshot",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}},
			},
		},
	}

	restore, err := manager.BuildPrepareDataRestore(comp, backup, nil)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() error = %v", err)
	}
	if restore.Spec.Backup.SourceTargetName != "target-b" {
		t.Fatalf("source target name = %q, want target-b", restore.Spec.Backup.SourceTargetName)
	}
	policy := restore.Spec.PrepareDataConfig.RequiredPolicyForAllPodSelection
	if policy == nil || policy.DataRestorePolicy != dpv1alpha1.OneToOneRestorePolicy {
		t.Fatalf("required policy = %#v, want one-to-one", policy)
	}

	manager.SourceTargetName = ""
	restore, err = manager.BuildPrepareDataRestore(comp, backup, nil)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() with annotation fallback error = %v", err)
	}
	if restore.Spec.Backup.SourceTargetName != "target-a" {
		t.Fatalf("annotation fallback source target name = %q, want target-a", restore.Spec.Backup.SourceTargetName)
	}
}
