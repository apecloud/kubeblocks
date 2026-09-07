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
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestHScaleRejectsFlatFromBackupBeforeMutation(t *testing.T) {
	cluster := &appsv1.Cluster{Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{
		{Name: "ordinary", Replicas: 1}, {Name: "flat", Replicas: 1, FlatInstanceOrdinal: true},
	}}}
	ops := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{
		{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "ordinary"}, ScaleOut: &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}},
		{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "flat"}, ScaleOut: &opsv1alpha1.ScaleOut{FromBackup: &opsv1alpha1.FromBackup{Name: "backup"}}},
	}}}}
	before := cluster.DeepCopy()
	// A nil client also guarantees validation precedes updates to earlier Ops.
	err := (horizontalScalingOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, nil, &OpsResource{Cluster: cluster, OpsRequest: ops})
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("want Fatal, got %v", err)
	}
	if !reflect.DeepEqual(before, cluster) {
		t.Fatal("unsupported request mutated cluster")
	}
}

func TestHScaleRollbackParticipants(t *testing.T) {
	source := map[string]string{"demo-db-7": "", "demo-db-42": "large"}
	details := []opsv1alpha1.ProgressStatusDetail{
		{Group: "db/Create", ObjectKey: "Pod/demo-db-99"},
		{Group: "db/Delete", ObjectKey: "Pod/demo-db-42"},
		{Group: "other/Create", ObjectKey: "Pod/other-0"},
		{Group: "db/OtherAction", ObjectKey: "Pod/demo-db-100"},
		{Group: "db/Create", ObjectKey: "PVC/data-demo-db-99"},
	}
	created, deleted := rollbackInstanceSets(source, details, "db")
	if !reflect.DeepEqual(created, source) ||
		!reflect.DeepEqual(deleted, map[string]string{"demo-db-99": ""}) {
		t.Fatalf("rollback must check source health and only recorded deletions: created=%v deleted=%v", created, deleted)
	}
	// With no recorded participants, current runtime objects must not be
	// guessed to be operation-created instances. Source health is still checked.
	created, deleted = rollbackInstanceSets(source, nil, "db")
	if !reflect.DeepEqual(created, source) || len(deleted) != 0 {
		t.Fatalf("empty progress lost source health or invented deletions: created=%v deleted=%v", created, deleted)
	}
	delete(created, "demo-db-7")
	if len(source) != 2 {
		t.Fatal("rollback mutated the saved source")
	}
}

func TestHorizontalDiff(t *testing.T) {
	source := map[string]string{"demo-0": "", "demo-1": "big"}
	target := map[string]string{"demo-0": "", "demo-2": "big"}
	created, deleted := diffAssignments(source, target)
	if len(created) != 1 || created["demo-2"] != "big" || len(deleted) != 1 || deleted["demo-1"] != "big" {
		t.Fatalf("unexpected assignment diff: created=%#v deleted=%#v", created, deleted)
	}
	horizontalScaling := opsv1alpha1.HorizontalScaling{
		ScaleOut: &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-2"}},
		ScaleIn:  &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-1"}},
	}
	if !horizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("expected explicit online/offline transition to match diff")
	}
	horizontalScaling.ScaleOut.OfflineInstancesToOnline = []string{"demo-3"}
	if horizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("stale allocation must not satisfy an explicit identity transition")
	}
	if horizontalDiffMatchesOperation(opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{}}, created, deleted) {
		t.Fatal("scale-out-only operation must not accept an unexpected deletion")
	}
}

// Exercise the real Operations runtime and cancellation handler, without a
// forward ReconcileAction publishing progress first. Desired allocation alone
// must neither complete rollback nor make unrelated objects participants.
func TestNonFlatHScaleCancellation(t *testing.T) {
	for _, operation := range []string{"scale-out", "scale-in", "named-swap", "online-high-ordinal", "from-backup"} {
		for _, unrelatedReleased := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/unrelated-released=%t", operation, unrelatedReleased), func(t *testing.T) {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme, dpv1alpha1.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				comp := appsv1.ClusterComponentSpec{Name: "db", ComponentDef: "database", Replicas: 2}
				source := map[string]string{"demo-db-0": "", "demo-db-1": ""}
				scaling := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}}
				var restored, removed []string
				offline := map[string]string{}
				switch operation {
				case "scale-out", "from-backup":
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					removed = []string{"demo-db-2"}
					if operation == "from-backup" {
						// No Backup exists in the client: cancellation must not invoke
						// restore or depend on its state.
						scaling.ScaleOut.FromBackup = &opsv1alpha1.FromBackup{Name: "not-needed-for-cancel"}
					}
				case "scale-in":
					scaling.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
					restored = []string{"demo-db-1"}
				case "named-swap":
					source = map[string]string{"demo-db-0": "", "demo-db-large-0": "large"}
					comp.Instances = []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(1)}}
					comp.OfflineInstances = []string{"demo-db-large-1"}
					offline["demo-db-large-1"] = "large"
					scaling.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-db-large-0"}}
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-large-1"}}
					restored, removed = []string{"demo-db-large-0"}, []string{"demo-db-large-1"}
				case "online-high-ordinal":
					comp.Replicas = 1
					source = map[string]string{"demo-db-0": ""}
					comp.OfflineInstances = []string{"demo-db-5"}
					offline["demo-db-5"] = ""
					scaling.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-5"}}
					// Action adds one replica. The non-flat allocator selects 1,
					// not 5; rollback must reverse the actual forward configuration.
					removed = []string{"demo-db-1"}
				}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
					Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
					Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase,
						Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "cancel", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{scaling}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
					Status: workloads.InstanceSetStatus{CurrentRevisions: map[string]string{"demo-db-99": "r"}}}
				for name, template := range source {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: name, TemplateName: templateName(template), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions[name] = "r"
				}
				for name, template := range offline {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
						PodName: name, TemplateName: templateName(template), DesiredState: workloads.InstanceDesiredStateOffline})
				}
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
					PodName: "demo-db-99", DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating})
				if !unrelatedReleased {
					delete(its.Status.CurrentRevisions, "demo-db-99")
					its.Status.InstanceStatus = its.Status.InstanceStatus[:len(its.Status.InstanceStatus)-1]
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(cluster, ops, its,
					&appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}).Build()
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100),
					Runtimes: map[string]OpsRuntime{"db": newOpsRuntime(ctx, cli, "")}}
				h, req := horizontalScalingOpsHandler{}, intctrlutil.RequestCtx{Ctx: ctx}
				if err := h.SaveLastConfiguration(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				if err := h.Action(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if len(ops.Status.Components) != 0 {
					t.Fatal("test requires cancellation without forward progress records")
				}
				if err := h.Cancel(req, cli, res); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(cluster.Spec.ComponentSpecs[0], comp) {
					t.Fatal("cancel did not restore the original component configuration")
				}
				ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				for _, name := range removed {
					its.Status.CurrentRevisions[name] = "r"
				}
				conditions := func(kind workloads.ConditionType, names []string) {
					t.Helper()
					message, err := json.Marshal(names)
					if err != nil {
						t.Fatal(err)
					}
					its.Status.Conditions = []metav1.Condition{{Type: string(kind), Status: metav1.ConditionFalse,
						Message: string(message)}}
				}
				check := func(want opsv1alpha1.OpsPhase) {
					t.Helper()
					if err := cli.Status().Update(ctx, its); err != nil {
						t.Fatal(err)
					}
					phase, _, err := h.ReconcileAction(req, cli, res)
					if err != nil || phase != want {
						t.Fatalf("phase=%s, want=%s, err=%v, progress=%s", phase, want, err, ops.Status.Progress)
					}
					for _, detail := range ops.Status.Components["db"].ProgressDetails {
						if detail.ObjectKey == "Pod/demo-db-99" || detail.ObjectKey == "Pod/demo-db-0" {
							t.Fatalf("unrelated instance became a cancellation participant: %#v", detail)
						}
					}
				}
				// Restored names already have revisions, but may still be unready.
				conditions(workloads.InstanceReady, restored)
				check(opsv1alpha1.OpsRunningPhase)
				for _, name := range removed {
					delete(its.Status.CurrentRevisions, name)
				}
				if len(restored) > 0 {
					check(opsv1alpha1.OpsRunningPhase)
					conditions(workloads.InstanceAvailable, restored)
					check(opsv1alpha1.OpsRunningPhase)
					conditions(workloads.InstanceFailure, restored)
					check(opsv1alpha1.OpsFailedPhase)
				}
				// Unchanged source 0 is also unready: as before this PR, only the
				// affected identities control cancellation completion.
				conditions(workloads.InstanceReady, []string{"demo-db-0", "demo-db-99"})
				check(opsv1alpha1.OpsSucceedPhase)
				if ops.Status.Progress != fmt.Sprintf("%d/%d", len(restored)+len(removed), len(restored)+len(removed)) {
					t.Fatalf("unexpected cancellation progress: %s", ops.Status.Progress)
				}
			})
		}
	}
}

func TestHScaleInstanceStatusLifecycle(t *testing.T) {
	for _, flat := range []bool{false, true} {
		for _, operation := range []string{"scale-out", "scale-in", "offline-online", "cancel-out", "cancel-in"} {
			t.Run(fmt.Sprintf("%s/flat=%v", operation, flat), func(t *testing.T) {
				ctx := context.Background()
				scheme := runtime.NewScheme()
				for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
					if err := add(scheme); err != nil {
						t.Fatal(err)
					}
				}
				const componentName = "db"
				defaultName, largeName, offlineName := "demo-db-7", "demo-db-42", "demo-db-8"
				newDefaultName, laterDefaultName := "demo-db-99", "demo-db-98"
				if !flat {
					// Non-flat names must obey the template naming rule. Flat
					// allocations deliberately retain non-contiguous ordinals.
					defaultName, largeName, offlineName = "demo-db-0", "demo-db-large-0", "demo-db-large-1"
					newDefaultName, laterDefaultName = "demo-db-1", "demo-db-2"
				}
				source := []workloads.InstanceStatus{
					{PodName: defaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: largeName, TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: offlineName, TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateOffline},
					{PodName: "demo-db-9", DesiredState: workloads.InstanceDesiredStateOffline},
				}
				comp := appsv1.ClusterComponentSpec{Name: componentName, ComponentDef: "database", Replicas: 2,
					FlatInstanceOrdinal: flat, Instances: []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(1)}},
					OfflineInstances: []string{offlineName, "demo-db-9"}}
				cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
					Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
					Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase, Components: map[string]appsv1.ClusterComponentStatus{componentName: {Phase: appsv1.RunningComponentPhase}}}}
				hs := opsv1alpha1.HorizontalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName}}
				switch operation {
				case "scale-out", "cancel-out":
					hs.ScaleOut = &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}}
				case "scale-in", "cancel-in":
					hs.ScaleIn = &opsv1alpha1.ScaleIn{ReplicaChanger: opsv1alpha1.ReplicaChanger{Instances: []opsv1alpha1.InstanceReplicasTemplate{{Name: "large", ReplicaChanges: 1}}}}
				case "offline-online":
					hs.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{largeName}}
					hs.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{offlineName}}
				}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "hscale", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{hs}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
					Status: workloads.InstanceSetStatus{InstanceStatus: source, CurrentRevisions: map[string]string{defaultName: "r", largeName: "r"}}}
				cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(cluster, ops, its,
					&appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}).Build()
				res := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(100), Runtimes: map[string]OpsRuntime{componentName: newOpsRuntime(ctx, cli, "")}}
				h := horizontalScalingOpsHandler{}
				req := intctrlutil.RequestCtx{Ctx: ctx}
				its.Status.InstanceStatus = nil
				if err := cli.Status().Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				if err := h.SaveLastConfiguration(req, cli, res); !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
					t.Fatalf("unpublished source allocation must wait: %v", err)
				}
				its.Status.InstanceStatus = source
				if err := cli.Status().Update(ctx, its); err != nil {
					t.Fatal(err)
				}
				if err := h.SaveLastConfiguration(req, cli, res); err != nil {
					t.Fatal(err)
				}
				// OpsManager persists LastConfiguration before invoking Action.
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				saved := ops.DeepCopy().Status.LastConfiguration
				assignments := saved.Components[componentName].SourceInstanceAssignments
				want := 2
				if operation == "offline-online" {
					want = 3
				}
				if len(assignments) != want {
					t.Fatalf("unexpected captured assignments: %#v", assignments)
				}
				if err := h.Action(req, cli, res); err != nil {
					t.Fatal(err)
				}
				check := func(want opsv1alpha1.OpsPhase) {
					t.Helper()
					phase, _, err := h.ReconcileAction(req, cli, res)
					if err != nil {
						t.Fatal(err)
					}
					if phase != want {
						t.Fatalf("phase=%s progress=%s want=%s details=%#v", phase, ops.Status.Progress, want, ops.Status.Components)
					}
					if !reflect.DeepEqual(saved, ops.Status.LastConfiguration) {
						t.Fatal("reconcile changed source configuration")
					}
				}
				publish := func() {
					t.Helper()
					if err := cli.Status().Update(ctx, its); err != nil {
						t.Fatal(err)
					}
				}
				check(opsv1alpha1.OpsRunningPhase) // Neither stale identities nor equal-count stale swaps may complete.
				if operation == "scale-out" {
					// A later direct spec edit and its allocation are not this
					// operation's target, even when they agree with each other.
					cluster.Spec.ComponentSpecs[0].Replicas++
					its.Status.InstanceStatus = append(append([]workloads.InstanceStatus(nil), source...),
						workloads.InstanceStatus{PodName: laterDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
						workloads.InstanceStatus{PodName: newDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions[laterDefaultName] = "r"
					its.Status.CurrentRevisions[newDefaultName] = "r"
					publish()
					check(opsv1alpha1.OpsRunningPhase)
					cluster.Spec.ComponentSpecs[0].Replicas--
					delete(its.Status.CurrentRevisions, laterDefaultName)
					delete(its.Status.CurrentRevisions, newDefaultName)
				}
				target := append([]workloads.InstanceStatus(nil), source...)
				switch operation {
				case "scale-out", "cancel-out":
					target = append(target, workloads.InstanceStatus{PodName: newDefaultName, TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
				case "scale-in", "cancel-in":
					target[1].DesiredState = workloads.InstanceDesiredStateReleased
				case "offline-online":
					target[1].DesiredState = workloads.InstanceDesiredStateOffline
					target[2].DesiredState = workloads.InstanceDesiredStateActive
				}
				its.Status.InstanceStatus = target
				publish()
				check(opsv1alpha1.OpsRunningPhase) // Allocation alone does not create/delete actual instances.
				switch operation {
				case "scale-out", "cancel-out":
					its.Status.CurrentRevisions[newDefaultName] = "r"
				case "scale-in", "cancel-in":
					delete(its.Status.CurrentRevisions, largeName)
				case "offline-online":
					delete(its.Status.CurrentRevisions, largeName)
					its.Status.CurrentRevisions[offlineName] = "r"
				}
				publish()
				if operation != "cancel-out" && operation != "cancel-in" {
					check(opsv1alpha1.OpsSucceedPhase)
					return
				}
				ops.Spec.Cancel = true
				ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
				if err := cli.Status().Update(ctx, ops); err != nil {
					t.Fatal(err)
				}
				if err := h.Cancel(req, cli, res); err != nil {
					t.Fatal(err)
				}
				check(opsv1alpha1.OpsRunningPhase) // Target still describes the forward operation.
				its.Status.InstanceStatus = append([]workloads.InstanceStatus(nil), source...)
				if operation == "cancel-out" {
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: newDefaultName, DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating})
				}
				publish()
				check(opsv1alpha1.OpsRunningPhase) // Restored allocation is not proof that rollback has finished.
				if operation == "cancel-out" {
					delete(its.Status.CurrentRevisions, newDefaultName)
					its.Status.InstanceStatus = source
				} else {
					its.Status.CurrentRevisions[largeName] = "r"
				}
				publish()
				check(opsv1alpha1.OpsSucceedPhase)
			})
		}
	}
}
