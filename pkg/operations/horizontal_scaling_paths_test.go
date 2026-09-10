/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
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
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// Use the real naming and workload runtime; only API storage is in memory.
type horizontalScalingFixture struct {
	cli                                      client.Client
	res                                      *OpsResource
	req                                      intctrlutil.RequestCtx
	clusterWrites, backupReads, restoreReads int
}

// Observe the real runtime and inject failures only to check short-circuit order.
type horizontalScalingRuntimeTrace struct {
	OpsRuntime
	calls  []string
	specs  []appsv1.ClusterComponentSpec
	failAt int
	err    error
}

func (r *horizontalScalingRuntimeTrace) GenerateInstanceNameSet(clusterName, compName string, replicas int32,
	instances []appsv1.InstanceTemplate, offline []string) (map[string]string, error) {
	r.calls = append(r.calls, fmt.Sprintf("names(%s,%d)", compName, replicas))
	spec := appsv1.ClusterComponentSpec{Name: compName, Replicas: replicas, Instances: instances, OfflineInstances: offline}
	r.specs = append(r.specs, *spec.DeepCopy())
	if r.failAt > 0 && len(r.calls) == r.failAt {
		return nil, r.err
	}
	return r.OpsRuntime.GenerateInstanceNameSet(clusterName, compName, replicas, instances, offline)
}

func (r *horizontalScalingRuntimeTrace) GetWorkload(namespace, clusterName, compName string) (Workload, error) {
	r.calls = append(r.calls, "workload("+compName+")")
	return r.OpsRuntime.GetWorkload(namespace, clusterName, compName)
}

func TestHorizontalScalingParticipantAndProgressOrder(t *testing.T) {
	for _, fromBackup := range []bool{false, true} {
		want := []string{"names(db,1)", "names(db,1)", "names(db,2)", "workload(db)"}
		if fromBackup {
			want = append([]string{"names(db,1)", "names(db,1)", "names(db,1)", "names(db,2)"}, want...)
		}
		// Fail each name-planning step in turn to ensure no later planning,
		// workload checks, or target writes occur after the first error.
		for failAt := 0; failAt < len(want); failAt++ {
			t.Run(fmt.Sprintf("backup=%t/failAt=%d", fromBackup, failAt), func(t *testing.T) {
				f := newHorizontalScalingFixture(t, scaleOutRequest("db", fromBackup))
				if fromBackup {
					f.addBackup(t)
				}
				trace := &horizontalScalingRuntimeTrace{OpsRuntime: f.res.Runtimes["db"], err: errors.New("name planning failed")}
				f.res.Runtimes["db"] = trace
				hs := horizontalScalingOpsHandler{}
				if err := hs.Action(f.req, f.cli, f.res); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(trace.calls, []string{"names(db,1)"}) {
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
				if fromBackup && (failAt == 0 || failAt > 4) {
					wantRestores = 1
				}
				if len(restores.Items) != wantRestores {
					t.Fatalf("restores = %d, want %d", len(restores.Items), wantRestores)
				}
			})
		}
	}
}

func TestHorizontalScalingOnlineInferenceInputs(t *testing.T) {
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
			request := scaleOutRequest("db", false)
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
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
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
			f.replicas(t, "db", tc.wantReplicas)
			if tc.template && (len(spec.Instances) != 1 || spec.Instances[0].GetReplicas() != 2) {
				t.Fatalf("target instances = %+v", spec.Instances)
			}
			if !tc.empty && len(spec.OfflineInstances) != 0 {
				t.Fatalf("target offline = %v", spec.OfflineInstances)
			}
		})
	}
}

func newHorizontalScalingFixture(t *testing.T, requests ...opsv1alpha1.HorizontalScaling) *horizontalScalingFixture {
	t.Helper()
	f := &horizontalScalingFixture{}
	f.req = intctrlutil.RequestCtx{Ctx: context.Background(), Recorder: record.NewFakeRecorder(100)}
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme,
		dpv1alpha1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "cluster1"},
		Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase}}
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default", UID: "opsreq123"},
		Spec: opsv1alpha1.OpsRequestSpec{Type: opsv1alpha1.HorizontalScalingType,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: requests}},
		Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
	objects := []client.Object{cluster, ops, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}}
	for _, request := range requests {
		spec := appsv1.ClusterComponentSpec{Name: request.ComponentName, ComponentDef: "database", Replicas: 1,
			VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}}
		cluster.Spec.ComponentSpecs = append(cluster.Spec.ComponentSpecs, spec)
		comp, err := component.BuildComponent(cluster, &spec, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		objects = append(objects, comp)
	}
	f.cli = fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).
		WithStatusSubresource(ops, &dpv1alpha1.Restore{}).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(ctx context.Context, cli client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			switch obj.(type) {
			case *dpv1alpha1.Backup:
				f.backupReads++
			case *dpv1alpha1.Restore:
				f.restoreReads++
			}
			return cli.Get(ctx, key, obj, opts...)
		},
		Update: func(ctx context.Context, cli client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*appsv1.Cluster); ok {
				f.clusterWrites++
			}
			return cli.Update(ctx, obj, opts...)
		},
	}).Build()
	f.res = &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: f.req.Recorder}
	var err error
	f.res.Runtimes, err = buildOpsRuntimes(f.req.Ctx, f.cli, f.res)
	if err != nil {
		t.Fatal(err)
	}
	if err := (horizontalScalingOpsHandler{}).SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	// OpsManager persists this snapshot before invoking Action/ReconcileAction.
	if err := f.cli.Status().Update(f.req.Ctx, ops); err != nil {
		t.Fatal(err)
	}
	return f
}

func scaleOutRequest(name string, backup bool) opsv1alpha1.HorizontalScaling {
	r := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: name},
		ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}}
	if backup {
		r.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "snapshot"}
	}
	return r
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

func (f *horizontalScalingFixture) replicas(t *testing.T, name string, want int32) {
	t.Helper()
	cluster := &appsv1.Cluster{}
	if err := f.cli.Get(f.req.Ctx, client.ObjectKeyFromObject(f.res.Cluster), cluster); err != nil {
		t.Fatal(err)
	}
	if got := cluster.Spec.GetComponentByName(name).Replicas; got != want {
		t.Fatalf("%s replicas = %d, want %d", name, got, want)
	}
}

func (f *horizontalScalingFixture) reconcile(t *testing.T, want opsv1alpha1.OpsPhase) {
	t.Helper()
	phase, _, err := (horizontalScalingOpsHandler{}).ReconcileAction(f.req, f.cli, f.res)
	if err != nil {
		t.Fatal(err)
	}
	if phase != want {
		t.Fatalf("reconcile phase = %s, want %s", phase, want)
	}
}

func TestHorizontalScalingOrdinaryPathDoesNotRestore(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.replicas(t, "db", 2)
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	if f.backupReads != 0 || f.restoreReads != 0 || f.clusterWrites != 1 {
		t.Fatalf("ordinary path API calls: backup=%d restore=%d cluster writes=%d", f.backupReads, f.restoreReads, f.clusterWrites)
	}
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-1" || details[0].Status != opsv1alpha1.PendingProgressStatus {
		t.Fatalf("unexpected create progress: %+v", f.res.OpsRequest.Status)
	}
}

func TestHorizontalScalingMixedComponentsRestoreTiming(t *testing.T) {
	for _, backupFirst := range []bool{false, true} {
		name := "ordinary-first"
		if backupFirst {
			name = "backup-first"
		}
		t.Run(name, func(t *testing.T) {
			requests := []opsv1alpha1.HorizontalScaling{scaleOutRequest("ordinary", false), scaleOutRequest("restored", true)}
			if backupFirst {
				requests[0], requests[1] = requests[1], requests[0]
			}
			f := newHorizontalScalingFixture(t, requests...)
			f.addBackup(t)
			if err := (horizontalScalingOpsHandler{}).Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			f.replicas(t, "ordinary", 2)
			f.replicas(t, "restored", 1)
			if f.backupReads != 0 || f.restoreReads != 0 {
				t.Fatal("Action must defer Restore work")
			}
			for i := 0; i < 2; i++ {
				f.reconcile(t, opsv1alpha1.OpsRunningPhase)
				f.replicas(t, "restored", 1)
				if f.clusterWrites != 1 {
					t.Fatal("pending Restore wrote target configuration")
				}
				if got := f.res.OpsRequest.Status.Components["restored"].Message; got != "Restore Data In Progress" {
					t.Fatalf("message = %q", got)
				}
				if got := f.res.OpsRequest.Status.Progress; got != "0/2" {
					t.Fatalf("progress = %q", got)
				}
			}
			restores := &dpv1alpha1.RestoreList{}
			if err := f.cli.List(f.req.Ctx, restores); err != nil {
				t.Fatal(err)
			}
			if len(restores.Items) != 1 {
				t.Fatalf("repeated reconcile created %d restores", len(restores.Items))
			}
			restore := &restores.Items[0]
			if restore.Spec.PrepareDataConfig.RestoreVolumeClaimsTemplate.StartingIndex != 1 {
				t.Fatal("unexpected restore ordinal")
			}
			restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
			if err := f.cli.Status().Update(f.req.Ctx, restore); err != nil {
				t.Fatal(err)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			f.replicas(t, "restored", 2)
			if f.clusterWrites != 2 || f.res.OpsRequest.Status.Components["restored"].Message != "Restore Data Completed" {
				t.Fatal("missing restore completion write/message")
			}
			reads := f.backupReads + f.restoreReads
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if f.clusterWrites != 2 || reads != f.backupReads+f.restoreReads {
				t.Fatal("completed marker did not skip repeated restore work")
			}
			// Restore completion alone does not complete the Ops: the newly
			// requested instances must become available on both paths.
			for _, name := range []string{"ordinary", "restored"} {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name + "-1", Namespace: "default",
					Labels: constant.GetCompLabels("demo", name)},
					Status: corev1.PodStatus{Phase: corev1.PodRunning,
						Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}}
				if err := f.cli.Create(f.req.Ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if got := f.res.OpsRequest.Status.Progress; got != "2/2" {
				t.Fatalf("completed progress = %q", got)
			}
			for _, name := range []string{"ordinary", "restored"} {
				details := f.res.OpsRequest.Status.Components[name].ProgressDetails
				if len(details) != 1 || details[0].ObjectKey != "Pod/demo-"+name+"-1" || details[0].Status != opsv1alpha1.SucceedProgressStatus {
					t.Fatalf("unexpected completed participants for %s: %+v", name, details)
				}
			}
			f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
			if f.clusterWrites != 2 || reads != f.backupReads+f.restoreReads {
				t.Fatal("completed Ops repeated restore work")
			}
		})
	}
}

func TestHorizontalScalingRestoreFailureAndCancellation(t *testing.T) {
	t.Run("failed-restore", func(t *testing.T) {
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
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
		f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
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

func TestHorizontalScalingCancelRestoresConfigurationAndDirection(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", false))
	spec := &f.res.Cluster.Spec.ComponentSpecs[0]
	spec.Instances = []appsv1.InstanceTemplate{{Name: "foo", Replicas: pointer.Int32(1)}}
	spec.OfflineInstances = []string{"demo-db-foo-0"}
	hs := horizontalScalingOpsHandler{}
	if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	last := f.res.OpsRequest.Status.LastConfiguration.Components["db"]
	if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
		t.Fatal(err)
	}
	if err := hs.Action(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	f.reconcile(t, opsv1alpha1.OpsRunningPhase)
	details := f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "0/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-0" ||
		details[0].Status != opsv1alpha1.PendingProgressStatus || !strings.Contains(details[0].Message, "create") {
		t.Fatalf("unexpected create participants: %+v", f.res.OpsRequest.Status)
	}
	f.res.OpsRequest.Status.Phase = opsv1alpha1.OpsCancellingPhase
	if err := hs.Cancel(f.req, f.cli, f.res); err != nil {
		t.Fatal(err)
	}
	if spec.Replicas != *last.Replicas || !reflect.DeepEqual(spec.Instances, last.Instances) || !reflect.DeepEqual(spec.OfflineInstances, last.OfflineInstances) {
		t.Fatal("Cancel did not restore the original configuration")
	}
	// ReconcileAction reports successful rollback; OpsManager maps it to Cancelled.
	f.reconcile(t, opsv1alpha1.OpsSucceedPhase)
	details = f.res.OpsRequest.Status.Components["db"].ProgressDetails
	if f.res.OpsRequest.Status.Progress != "1/1" || len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-0" || !strings.Contains(details[0].Message, "delete") {
		t.Fatalf("unexpected cancel progress: %+v", f.res.OpsRequest.Status)
	}
}

func TestHorizontalScalingShardCountAndShardReplicas(t *testing.T) {
	for _, changeCount := range []bool{false, true} {
		name := "replicas-per-shard"
		request := scaleOutRequest("sharded", false)
		if changeCount {
			name = "shard-count"
			request.ScaleOut = nil
			request.Shards = pointer.Int32(3)
		}
		t.Run(name, func(t *testing.T) {
			f := newHorizontalScalingFixture(t, request)
			cluster := f.res.Cluster
			cluster.Spec.Shardings = []appsv1.ClusterSharding{{Name: "sharded", Shards: 2, Template: cluster.Spec.ComponentSpecs[0]}}
			cluster.Spec.ComponentSpecs = nil
			for _, shard := range []string{"sharded-a", "sharded-b"} {
				comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + shard, Namespace: "default",
					Labels: constant.GetCompLabels("demo", shard, map[string]string{constant.KBAppShardingNameLabelKey: "sharded"})}}
				if err := f.cli.Create(f.req.Ctx, comp); err != nil {
					t.Fatal(err)
				}
			}
			hs := horizontalScalingOpsHandler{}
			if err := hs.SaveLastConfiguration(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			if err := f.cli.Status().Update(f.req.Ctx, f.res.OpsRequest); err != nil {
				t.Fatal(err)
			}
			if err := hs.Action(f.req, f.cli, f.res); err != nil {
				t.Fatal(err)
			}
			wantShards, wantReplicas, wantProgress := int32(2), int32(2), "0/2"
			if changeCount {
				wantShards, wantReplicas, wantProgress = 3, 1, "0/1"
			}
			if got := cluster.Spec.Shardings[0]; got.Shards != wantShards || got.Template.Replicas != wantReplicas {
				t.Fatalf("unexpected sharding: %+v", got)
			}
			f.reconcile(t, opsv1alpha1.OpsRunningPhase)
			if got := f.res.OpsRequest.Status.Progress; got != wantProgress {
				t.Fatalf("progress = %s, want %s", got, wantProgress)
			}
			details := f.res.OpsRequest.Status.Components["sharded"].ProgressDetails
			if len(details) != 2 {
				t.Fatalf("progress details = %+v", details)
			}
			for _, detail := range details {
				prefix := "Pod/demo-sharded-"
				if changeCount {
					prefix = "Component/demo-sharded-"
				}
				if !strings.HasPrefix(detail.ObjectKey, prefix) {
					t.Fatalf("wrong progress path: %+v", detail)
				}
			}
			if f.backupReads != 0 || f.restoreReads != 0 {
				t.Fatal("sharding request entered Restore path")
			}
		})
	}
}

// Characterize main's cancellation behavior; fixing it belongs in a separate change.
func TestHorizontalScalingCancellingBackupStillSubmitsTarget(t *testing.T) {
	f := newHorizontalScalingFixture(t, scaleOutRequest("db", true))
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
