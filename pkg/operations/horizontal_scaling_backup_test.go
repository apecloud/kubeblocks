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

package operations

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/plan"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestHorizontalScalingCreateRestorePreservesPerPodStartingIndex(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	for _, addToScheme := range []func(*runtime.Scheme) error{
		corev1.AddToScheme,
		appsv1.AddToScheme,
		dpv1alpha1.AddToScheme,
		opsv1alpha1.AddToScheme,
	} {
		if err := addToScheme(scheme); err != nil {
			t.Fatalf("add scheme: %v", err)
		}
	}

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "default", UID: types.UID("cluster1")},
	}
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "scale-out-from-backup", Namespace: "default", UID: types.UID("opsreq1")},
	}
	backup := &dpv1alpha1.Backup{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-backup", Namespace: "default"},
		Status: dpv1alpha1.BackupStatus{
			Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{
				Name:          "snapshot",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}},
			},
			Targets: []dpv1alpha1.BackupStatusTarget{{
				BackupTarget: dpv1alpha1.BackupTarget{
					Name:        "redis",
					PodSelector: &dpv1alpha1.PodSelector{Strategy: dpv1alpha1.PodSelectionStrategyAll},
				},
				SelectedTargetPods: []string{"redis-az-a-4", "redis-az-a-3"},
			}},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, opsRequest, backup).Build()
	opsRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest}
	synthesizedComponent := &component.SynthesizedComponent{
		Name: "redis",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
			ObjectMeta: metav1.ObjectMeta{Name: "data"},
		}},
	}
	componentSpec := &appsv1.ClusterComponentSpec{
		Name: "redis",
		Instances: []appsv1.InstanceTemplate{{
			Name: "az-a",
			Ordinals: appsv1.Ordinals{
				Ranges: []appsv1.Range{{Start: 3, End: 4}},
			},
		}},
	}
	for _, ordinal := range []int32{3, 4} {
		restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
			constant.OpsRequestNameLabelKey: opsRequest.Name,
		}, 1, ordinal)
		err := horizontalScalingOpsHandler{}.createRestore(
			intctrlutil.RequestCtx{Ctx: ctx, Recorder: record.NewFakeRecorder(1)},
			cli, opsRes, synthesizedComponent, restoreMGR, componentSpec, backup, "az-a")
		if err != nil {
			t.Fatalf("create restore for ordinal %d: %v", ordinal, err)
		}
	}

	restoreList := &dpv1alpha1.RestoreList{}
	if err := cli.List(ctx, restoreList, client.InNamespace("default")); err != nil {
		t.Fatalf("list restores: %v", err)
	}
	if len(restoreList.Items) != 2 {
		t.Fatalf("expected two restores, got %d", len(restoreList.Items))
	}
	restoresByOrdinal := map[int32]dpv1alpha1.Restore{}
	for i := range restoreList.Items {
		restore := restoreList.Items[i]
		startingIndex := restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.StartingIndex
		restoresByOrdinal[startingIndex] = restore
	}
	for _, ordinal := range []int32{3, 4} {
		restore, ok := restoresByOrdinal[ordinal]
		if !ok {
			t.Fatalf("missing restore for actual scale-out pod ordinal %d", ordinal)
		}
		claimTemplate := restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate
		if claimTemplate.Replicas != 1 {
			t.Fatalf("restore replicas = %d, want 1 for ordinal %d", claimTemplate.Replicas, ordinal)
		}
		if got := claimTemplate.Templates[0].Labels[constant.KBAppInstanceTemplateLabelKey]; got != "az-a" {
			t.Fatalf("instance template label = %q, want %q for ordinal %d", got, "az-a", ordinal)
		}
	}
}

func TestHorizontalScalingCreateRestoreReturnsFatalWhenNoRestoreBuilt(t *testing.T) {
	testCases := []struct {
		name         string
		backupMethod *dpv1alpha1.BackupMethod
		expectError  string
	}{
		{
			name:         "completed backup without backup method",
			backupMethod: nil,
			expectError:  "status.backupMethod",
		}, {
			// logical backups (e.g. TiDB BR) have no targetVolumes at all
			name:         "backup method without target volumes",
			backupMethod: &dpv1alpha1.BackupMethod{Name: "br"},
			expectError:  "has no target volumes matching component",
		}, {
			// targetVolumes exist but none match the component's volume claim templates
			name: "backup method target volumes match no component volume",
			backupMethod: &dpv1alpha1.BackupMethod{
				Name: "br",
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{
					Volumes: []string{"other-data"},
				},
			},
			expectError: "has no target volumes matching component",
		}}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, addToScheme := range []func(*runtime.Scheme) error{
				corev1.AddToScheme,
				appsv1.AddToScheme,
				dpv1alpha1.AddToScheme,
				opsv1alpha1.AddToScheme,
			} {
				if err := addToScheme(scheme); err != nil {
					t.Fatalf("add scheme: %v", err)
				}
			}

			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "tidb",
					Namespace: "default",
					UID:       types.UID("cluster1"),
				},
			}
			opsRequest := &opsv1alpha1.OpsRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "scale-out-from-backup",
					Namespace: "default",
					UID:       types.UID("opsreq1"),
				},
			}
			backup := &dpv1alpha1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "br-full",
					Namespace: "default",
				},
				Status: dpv1alpha1.BackupStatus{
					Phase:        dpv1alpha1.BackupPhaseCompleted,
					BackupMethod: tc.backupMethod,
				},
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, opsRequest, backup).Build()
			opsRes := &OpsResource{
				Cluster:    cluster,
				OpsRequest: opsRequest,
			}
			synthesizedComponent := &component.SynthesizedComponent{
				Name: "tikv",
				VolumeClaimTemplates: []corev1.PersistentVolumeClaimTemplate{{
					ObjectMeta: metav1.ObjectMeta{Name: "data"},
					Spec: corev1.PersistentVolumeClaimSpec{
						AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					},
				}},
			}
			restoreMGR := plan.NewRestoreManager(ctx, cli, cluster, scheme, map[string]string{
				constant.OpsRequestNameLabelKey: opsRequest.Name,
			}, 1, 3)

			err := horizontalScalingOpsHandler{}.createRestore(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes,
				synthesizedComponent, restoreMGR, &appsv1.ClusterComponentSpec{Name: "tikv"}, backup, "")
			if err == nil {
				t.Fatal("expected fatal error when backup method cannot build prepareData restore")
			}
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
				t.Fatalf("expected fatal error, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.expectError) {
				t.Fatalf("unexpected error: %v", err)
			}

			restoreList := &dpv1alpha1.RestoreList{}
			if err := cli.List(ctx, restoreList, client.InNamespace("default")); err != nil {
				t.Fatalf("list restores: %v", err)
			}
			if len(restoreList.Items) != 0 {
				t.Fatalf("expected no restore to be created, got %d", len(restoreList.Items))
			}
		})
	}
}

func TestHorizontalScalingBackupParticipantAndProgressOrder(t *testing.T) {
	want := []string{"names(db,1)", "names(db,1)", "names(db,1)", "names(db,2)",
		"names(db,1)", "names(db,1)", "names(db,2)", "workload(db)"}
	// Fail each name-planning step in turn to ensure no later planning,
	// workload checks, or target writes occur after the first error.
	for failAt := 0; failAt < len(want); failAt++ {
		t.Run(fmt.Sprintf("failAt=%d", failAt), func(t *testing.T) {
			f := newHorizontalScalingFixture(t, backupScaleOutRequest("db"))
			f.saveConfiguration(t)
			f.addBackup(t)
			trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"], err: errors.New("name planning failed")}
			f.res.Runtimes["db"] = trace
			hs := horizontalScalingOpsHandler{}
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			wantAction := []string{"names(db,1)"}
			if !reflect.DeepEqual(trace.calls, wantAction) {
				t.Fatalf("Action calls = %v", trace.calls)
			}
			trace.calls, trace.specs, trace.failAt = nil, nil, failAt
			phase, _, err := hs.ReconcileAction(f.req, f.cli, f.res)
			if phase != opsv1alpha1.OpsRunningPhase {
				t.Fatalf("phase = %s", phase)
			}
			wantCalls := want
			if failAt > 0 {
				wantCalls = want[:failAt]
				if !errors.Is(err, trace.err) {
					t.Fatalf("error = %v, want injected error", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(trace.calls, wantCalls) {
				t.Fatalf("calls = %v, want %v", trace.calls, wantCalls)
			}
			if f.clusterWrites != 1 {
				t.Fatalf("cluster writes = %d", f.clusterWrites)
			}
			restores := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, restores); err != nil {
				t.Fatal(err)
			}
			wantRestores := 0
			if failAt == 0 || failAt > 4 {
				wantRestores = 1
			}
			if len(restores.Items) != wantRestores {
				t.Fatalf("restores = %d, want %d", len(restores.Items), wantRestores)
			}
		})
	}
}

func TestHorizontalScalingBackupOnlineInferenceInputs(t *testing.T) {
	for _, tc := range []struct {
		name                                             string
		template, explicitTotal, explicitTemplate, empty bool
		wantCalls                                        []string
		wantReplicas                                     int32
	}{
		{name: "infer-default", wantCalls: []string{"names(db,1)", "names(db,2)"}, wantReplicas: 2},
		{name: "infer-template", template: true, wantCalls: []string{"names(db,1)", "names(db,2)"}, wantReplicas: 2},
		{name: "explicit-total-skips-inference", explicitTotal: true, wantCalls: []string{"names(db,1)"}, wantReplicas: 2},
		{name: "explicit-template-keeps-lookup", template: true, explicitTemplate: true, wantCalls: []string{"names(db,1)", "names(db,1)"}, wantReplicas: 2},
		{name: "empty-list-skips-inference", empty: true, wantCalls: []string{"names(db,1)"}, wantReplicas: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := backupScaleOutRequest("db")
			request.ScaleOut.ReplicaChanges = nil
			instance := "demo-db-0"
			if tc.template {
				instance = "demo-db-foo-0"
			}
			if !tc.empty {
				request.ScaleOut.OfflineInstancesToOnline = []string{instance}
			}
			if tc.explicitTotal {
				request.ScaleOut.ReplicaChanges = pointer.Int32(1)
			}
			if tc.explicitTemplate {
				request.ScaleOut.Instances = []opsv1alpha1.InstanceReplicasTemplate{{Name: "foo", ReplicaChanges: 1}}
			}
			f := newHorizontalScalingFixture(t, request)
			spec := &f.res.Cluster.Spec.ComponentSpecs[0]
			spec.OfflineInstances = []string{instance}
			if tc.template {
				spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
			}
			hs := horizontalScalingOpsHandler{}
			if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			original := spec.DeepCopy()
			trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"]}
			f.res.Runtimes["db"] = trace
			replicas, instances, offline, err := hs.getExpectedCompValuesForRestore(f.res,
				f.res.OpsRequest.Status.LastConfiguration.Components["db"], request)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(trace.calls, tc.wantCalls) {
				t.Fatalf("calls = %v, want %v", trace.calls, tc.wantCalls)
			}
			if !reflect.DeepEqual(trace.specs[0].Instances, original.Instances) || !reflect.DeepEqual(trace.specs[0].OfflineInstances, original.OfflineInstances) {
				t.Fatalf("initial planning inputs = %+v", trace.specs[0])
			}
			if len(trace.specs) == 2 {
				candidate := trace.specs[1]
				if len(candidate.OfflineInstances) != 0 {
					t.Fatalf("candidate offline = %v", candidate.OfflineInstances)
				}
				if tc.template {
					want := int32(2)
					if tc.explicitTemplate {
						want = 1
					}
					if len(candidate.Instances) != 1 || candidate.Instances[0].GetReplicas() != want {
						t.Fatalf("candidate instances = %+v", candidate.Instances)
					}
				}
			}
			if replicas != tc.wantReplicas {
				t.Fatalf("target replicas = %d, want %d", replicas, tc.wantReplicas)
			}
			if !reflect.DeepEqual(spec, original) {
				t.Fatal("backup target calculation changed live configuration")
			}
			if tc.template && (len(instances) != 1 || instances[0].GetReplicas() != 2) {
				t.Fatalf("target instances = %+v", instances)
			}
			if !tc.empty && len(offline) != 0 {
				t.Fatalf("target offline = %v", offline)
			}
		})
	}
}

func TestHorizontalScalingRestoreFailureAndCancellation(t *testing.T) {
	t.Run("failed-restore", func(t *testing.T) {
		f := newHorizontalScalingFixture(t, backupScaleOutRequest("db"))
		f.saveConfiguration(t)
		f.addBackup(t)
		if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		f.reconcile(t, opsv1alpha1.OpsRunningPhase)
		restores := &dpv1alpha1.RestoreList{}
		if err := f.cli.List(f.req.Ctx, restores); err != nil {
			t.Fatal(err)
		}
		if len(restores.Items) != 1 {
			t.Fatalf("restores = %d", len(restores.Items))
		}
		restore := &restores.Items[0]
		restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
		if err := f.cli.Status().Update(f.req.Ctx, restore); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			_, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
			if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) || !strings.Contains(err.Error(), "restore for horizontalScaling failed") {
				t.Fatalf("unexpected error: %v", err)
			}
			f.replicas(t, "db", 1)
			if f.clusterWrites != 1 {
				t.Fatal("failed Restore wrote target configuration")
			}
		}
	})
	t.Run("cancelling-still-validates-backup", func(t *testing.T) {
		f := newHorizontalScalingFixture(t, backupScaleOutRequest("db"))
		f.saveConfiguration(t)
		f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
		if err := (horizontalScalingOpsHandler{}).Cancel(f.req, f.cli, f.res); err != nil {
			t.Fatal(err)
		}
		_, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
		if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) || !strings.Contains(err.Error(), "backup snapshot not found") {
			t.Fatalf("unexpected cancellation error: %v", err)
		}
		if f.backupReads != 1 {
			t.Fatal("Cancelling bypassed the existing backup path")
		}
	})
}

// Preserve the known main behavior here; the cancellation fix is outside this PR.
func TestHorizontalScalingCancellingBackupStillSubmitsTarget(t *testing.T) {
	f := newHorizontalScalingFixture(t, backupScaleOutRequest("db"))
	f.saveConfiguration(t)
	f.addBackup(t)
	hs := horizontalScalingOpsHandler{}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.replicas(t, "db", 1)
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	// Cancelling swaps the create/delete sets, so this pure scale-out request
	// has no restores to wait for and submits its original scale-out target.
	f.replicas(t, "db", 2)
	if f.res.OpsRequest.Status.Components["db"].Message != "Restore Data Completed" {
		t.Fatal("unexpected cancellation restore message")
	}
}

func backupScaleOutRequest(name string) opsv1alpha1.HorizontalScaling {
	request := scaleOutRequest(name)
	request.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "snapshot"}
	return request
}

func (f *horizontalScalingFixture) addBackup(t *testing.T) {
	t.Helper()
	backup := &dpv1alpha1.Backup{ObjectMeta: metav1.ObjectMeta{Name: "snapshot", Namespace: "default"},
		Status: dpv1alpha1.BackupStatus{Phase: dpv1alpha1.BackupPhaseCompleted,
			BackupMethod: &dpv1alpha1.BackupMethod{Name: "snapshot", SnapshotVolumes: pointer.Bool(true),
				TargetVolumes: &dpv1alpha1.TargetVolumeInfo{Volumes: []string{"data"}}},
			Targets: []dpv1alpha1.BackupStatusTarget{{BackupTarget: dpv1alpha1.BackupTarget{
				Name: "db", PodSelector: &dpv1alpha1.PodSelector{}}}}}}
	if err := f.cli.Create(f.req.Ctx, backup); err != nil {
		t.Fatal(err)
	}
}
