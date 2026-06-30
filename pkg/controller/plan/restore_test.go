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

func newRestoreManagerForTest() *RestoreManager {
	return &RestoreManager{
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "cluster",
				UID:       types.UID("12345678-1234-1234-1234-1234567890ab"),
			},
		},
		namespace:           "default",
		replicas:            2,
		startingIndex:       1,
		volumeRestorePolicy: dpv1alpha1.VolumeClaimRestorePolicyParallel,
		restoreLabels:       map[string]string{"restore": "true"},
	}
}

func TestBackupSourceTargetForRestore(t *testing.T) {
	statusTarget := &dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			Name: "status-target",
		},
	}
	tests := []struct {
		name     string
		backup   *dpv1alpha1.Backup
		wantName string
		wantNil  bool
	}{
		{name: "nil backup", wantNil: true},
		{
			name: "status target wins",
			backup: &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{Target: statusTarget},
			},
		},
		{
			name: "single target returns name",
			backup: &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{
					Targets: []dpv1alpha1.BackupStatusTarget{{
						BackupTarget: dpv1alpha1.BackupTarget{Name: "target-a"},
					}},
				},
			},
			wantName: "target-a",
		},
		{
			name: "multiple targets are ambiguous",
			backup: &dpv1alpha1.Backup{
				Status: dpv1alpha1.BackupStatus{
					Targets: []dpv1alpha1.BackupStatusTarget{
						{BackupTarget: dpv1alpha1.BackupTarget{Name: "target-a"}},
						{BackupTarget: dpv1alpha1.BackupTarget{Name: "target-b"}},
					},
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotTarget := backupSourceTargetForRestore(tt.backup)
			if gotName != tt.wantName {
				t.Fatalf("source target name = %q, want %q", gotName, tt.wantName)
			}
			if tt.wantNil && gotTarget != nil {
				t.Fatalf("source target = %#v, want nil", gotTarget)
			}
			if !tt.wantNil && gotTarget == nil {
				t.Fatal("source target is nil")
			}
		})
	}
}

func TestRestoreManagerBuildRequiredPolicy(t *testing.T) {
	manager := newRestoreManagerForTest()
	if got := manager.buildRequiredPolicy(nil); got != nil {
		t.Fatalf("nil source target policy = %#v, want nil", got)
	}
	if got := manager.buildRequiredPolicy(&dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAny},
		},
	}); got != nil {
		t.Fatalf("any strategy policy = %#v, want nil", got)
	}
	got := manager.buildRequiredPolicy(&dpv1alpha1.BackupStatusTarget{
		BackupTarget: dpv1alpha1.BackupTarget{
			PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll},
		},
	})
	if got == nil || got.DataRestorePolicy != dpv1alpha1.OneToOneRestorePolicy {
		t.Fatalf("unexpected all strategy policy: %#v", got)
	}
}

func TestRestoreManagerBuildSchedulingSpec(t *testing.T) {
	manager := newRestoreManagerForTest()
	comp := &component.SynthesizedComponent{
		PodSpec: &corev1.PodSpec{
			NodeSelector:  map[string]string{"disk": "ssd"},
			SchedulerName: "default-scheduler",
			NodeName:      "node-a",
			Tolerations:   []corev1.Toleration{{Key: "dedicated", Value: "db"}},
		},
	}

	got := manager.buildSchedulingSpec(comp, nil)
	if !reflect.DeepEqual(got.NodeSelector, comp.PodSpec.NodeSelector) ||
		got.SchedulerName != "default-scheduler" ||
		got.NodeName != "node-a" ||
		!reflect.DeepEqual(got.Tolerations, comp.PodSpec.Tolerations) {
		t.Fatalf("component scheduling spec not copied: %#v", got)
	}

	template := &appsv1.InstanceTemplate{
		SchedulingPolicy: &appsv1.SchedulingPolicy{
			NodeSelector:  map[string]string{"zone": "east"},
			SchedulerName: "template-scheduler",
			NodeName:      "node-b",
		},
	}
	got = manager.buildSchedulingSpec(comp, template)
	if !reflect.DeepEqual(got.NodeSelector, map[string]string{"zone": "east"}) ||
		got.SchedulerName != "template-scheduler" ||
		got.NodeName != "node-b" {
		t.Fatalf("template scheduling spec not preferred: %#v", got)
	}

	if got = manager.buildSchedulingSpec(&component.SynthesizedComponent{}, nil); !reflect.DeepEqual(got, dpv1alpha1.SchedulingSpec{}) {
		t.Fatalf("empty component scheduling spec = %#v, want empty", got)
	}
}

func TestRestoreManagerGetConnectionCredential(t *testing.T) {
	manager := newRestoreManagerForTest()
	if got := manager.getConnectionCredential(&component.SynthesizedComponent{}); got != nil {
		t.Fatalf("empty system accounts credential = %#v, want nil", got)
	}

	comp := &component.SynthesizedComponent{
		Name: "mysql",
		SystemAccounts: []appsv1.SystemAccount{
			{Name: "readonly"},
			{Name: "root", InitAccount: true},
		},
	}
	got := manager.getConnectionCredential(comp)
	if got == nil {
		t.Fatal("connection credential is nil")
		return
	}
	if got.SecretName != constant.GenerateAccountSecretName("cluster", "mysql", "root") ||
		got.UsernameKey != constant.AccountNameForSecret ||
		got.PasswordKey != constant.AccountPasswdForSecret {
		t.Fatalf("unexpected credential: %#v", got)
	}
}

func TestBuildPersistentVolumeClaimLabels(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{}
	BuildPersistentVolumeClaimLabels(&component.SynthesizedComponent{}, pvc, "data", "az-a")
	if pvc.Labels[constant.VolumeClaimTemplateNameLabelKey] != "data" ||
		pvc.Labels[constant.KBAppInstanceTemplateLabelKey] != "az-a" {
		t.Fatalf("unexpected pvc labels: %#v", pvc.Labels)
	}

	BuildPersistentVolumeClaimLabels(nil, pvc, "ignored", "ignored")
	BuildPersistentVolumeClaimLabels(&component.SynthesizedComponent{}, nil, "ignored", "ignored")
}

func TestRestoreManagerBuildPrepareDataRestore(t *testing.T) {
	manager := newRestoreManagerForTest()
	comp := &component.SynthesizedComponent{
		Name:     "mysql",
		Replicas: 2,
		Labels:   map[string]string{"component-label": "true"},
		StaticLabels: map[string]string{
			"static-label": "true",
		},
		DynamicLabels: map[string]string{
			"dynamic-label": "true",
		},
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			},
		}},
	}
	template := &appsv1.InstanceTemplate{
		Name: "az-a",
		Ordinals: appsv1.Ordinals{
			Ranges: []appsv1.Range{{Start: 3, End: 4}},
		},
		SchedulingPolicy: &appsv1.SchedulingPolicy{NodeName: "node-a"},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup"},
		Status: dpv1alpha1.BackupStatus{
			Targets: []dpv1alpha1.BackupStatusTarget{{
				BackupTarget: dpv1alpha1.BackupTarget{
					Name: "target-a",
					PodSelector: &dpv1alpha1.PodSelector{
						Strategy: dpv1alpha1.PodSelectionStrategyAll,
					},
				},
			}},
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name: "snapshot",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					Volumes: []string{"data"},
				},
			},
		},
	}

	restore, err := manager.BuildPrepareDataRestore(comp, backup, template)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() error = %v", err)
	}
	if restore == nil {
		t.Fatal("restore is nil")
		return
	}
	if restore.Spec.Backup.Name != "backup" ||
		restore.Spec.Backup.Namespace != "default" ||
		restore.Spec.Backup.SourceTargetName != "target-a" {
		t.Fatalf("unexpected backup ref: %#v", restore.Spec.Backup)
	}
	cfg := restore.Spec.PrepareDataConfig
	if cfg == nil {
		t.Fatal("prepare data config is nil")
		return
	}
	if cfg.RequiredPolicyForAllPodSelection == nil ||
		cfg.RequiredPolicyForAllPodSelection.DataRestorePolicy != dpv1alpha1.OneToOneRestorePolicy {
		t.Fatalf("unexpected required policy: %#v", cfg.RequiredPolicyForAllPodSelection)
	}
	if cfg.SchedulingSpec.NodeName != "node-a" {
		t.Fatalf("unexpected scheduling spec: %#v", cfg.SchedulingSpec)
	}
	if cfg.RestoreVolumeClaimsTemplate.Replicas != 2 ||
		cfg.RestoreVolumeClaimsTemplate.StartingIndex != 3 ||
		len(cfg.RestoreVolumeClaimsTemplate.Templates) != 1 {
		t.Fatalf("unexpected restore volume claim template: %#v", cfg.RestoreVolumeClaimsTemplate)
	}
	labels := cfg.RestoreVolumeClaimsTemplate.Templates[0].Labels
	if labels[constant.VolumeClaimTemplateNameLabelKey] != "data" ||
		labels[constant.KBAppInstanceTemplateLabelKey] != "az-a" ||
		labels["component-label"] != "true" ||
		labels["static-label"] != "true" ||
		labels["dynamic-label"] != "true" {
		t.Fatalf("unexpected restore pvc labels: %#v", labels)
	}
}

func TestRestoreManagerBuildPrepareDataRestoreWithoutVolumes(t *testing.T) {
	manager := newRestoreManagerForTest()
	comp := &component.SynthesizedComponent{
		Name: "mysql",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}

	restore, err := manager.BuildPrepareDataRestore(comp, &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup"},
		Status: dpv1alpha1.BackupStatus{
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name: "action",
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("BuildPrepareDataRestore() error = %v", err)
	}
	if restore != nil {
		t.Fatalf("restore = %#v, want nil", restore)
	}
}

func TestRestoreManagerBuildPrepareDataRestoreRequiresBackupMethod(t *testing.T) {
	manager := newRestoreManagerForTest()
	_, err := manager.BuildPrepareDataRestore(&component.SynthesizedComponent{}, &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "backup"},
	}, nil)
	if err == nil {
		t.Fatal("expected missing backup method error")
	}
}
