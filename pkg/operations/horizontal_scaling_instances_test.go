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

func TestHScaleRollbackWithoutForwardProgress(t *testing.T) {
	source := map[string]string{"demo-db-7": "", "demo-db-42": "large"}
	workload := &defaultWorkload{currentRevisionMap: map[string]string{
		"demo-db-7": "r", "demo-db-99": "r", "demo-db-8": "r",
	}}
	created, deleted := rollbackInstanceSets(source, workload, []string{"demo-db-8"}, nil, "db")
	if !reflect.DeepEqual(created, map[string]string{"demo-db-42": "large"}) ||
		!reflect.DeepEqual(deleted, map[string]string{"demo-db-99": ""}) {
		t.Fatalf("rollback lost unrecorded instances: created=%v deleted=%v", created, deleted)
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
				source := []workloads.InstanceStatus{
					{PodName: "demo-db-7", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "demo-db-42", TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "demo-db-8", TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateOffline},
					{PodName: "demo-db-9", DesiredState: workloads.InstanceDesiredStateOffline},
				}
				comp := appsv1.ClusterComponentSpec{Name: componentName, ComponentDef: "database", Replicas: 2,
					FlatInstanceOrdinal: flat, Instances: []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(1)}},
					OfflineInstances: []string{"demo-db-8", "demo-db-9"}}
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
					hs.ScaleIn = &opsv1alpha1.ScaleIn{OnlineInstancesToOffline: []string{"demo-db-42"}}
					hs.ScaleOut = &opsv1alpha1.ScaleOut{OfflineInstancesToOnline: []string{"demo-db-8"}}
				}
				ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "hscale", Namespace: "default"},
					Spec:   opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{hs}}},
					Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now().Add(-time.Minute))}}
				its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
					Status: workloads.InstanceSetStatus{InstanceStatus: source, CurrentRevisions: map[string]string{"demo-db-7": "r", "demo-db-42": "r"}}}
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
						workloads.InstanceStatus{PodName: "demo-db-98", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
						workloads.InstanceStatus{PodName: "demo-db-99", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
					its.Status.CurrentRevisions["demo-db-98"] = "r"
					its.Status.CurrentRevisions["demo-db-99"] = "r"
					publish()
					check(opsv1alpha1.OpsRunningPhase)
					cluster.Spec.ComponentSpecs[0].Replicas--
					delete(its.Status.CurrentRevisions, "demo-db-98")
					delete(its.Status.CurrentRevisions, "demo-db-99")
				}
				target := append([]workloads.InstanceStatus(nil), source...)
				switch operation {
				case "scale-out", "cancel-out":
					target = append(target, workloads.InstanceStatus{PodName: "demo-db-99", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
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
					its.Status.CurrentRevisions["demo-db-99"] = "r"
				case "scale-in", "cancel-in":
					delete(its.Status.CurrentRevisions, "demo-db-42")
				case "offline-online":
					delete(its.Status.CurrentRevisions, "demo-db-42")
					its.Status.CurrentRevisions["demo-db-8"] = "r"
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
					its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: "demo-db-99", DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating})
				}
				publish()
				check(opsv1alpha1.OpsRunningPhase) // Restored allocation is not proof that rollback has finished.
				if operation == "cancel-out" {
					delete(its.Status.CurrentRevisions, "demo-db-99")
					its.Status.InstanceStatus = source
				} else {
					its.Status.CurrentRevisions["demo-db-42"] = "r"
				}
				publish()
				check(opsv1alpha1.OpsSucceedPhase)
			})
		}
	}
}
