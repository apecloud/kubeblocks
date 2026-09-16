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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestVerticalScalingResultAndProgressConvergeIndependently(t *testing.T) {
	target := verticalScalingTestResources("2")
	component := appsv1.ClusterComponentSpec{
		Name: "db", Replicas: 2, Resources: target,
		Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1)}},
	}
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target,
	}
	ready := []workloads.InstanceStatus{
		verticalScalingTestInstance("demo-db-0", "", true),
		verticalScalingTestInstance("demo-db-reader-0", "reader", true),
	}

	for _, tc := range []struct {
		name       string
		phase      appsv1.ComponentPhase
		generation int64
		upToDate   bool
		instances  []workloads.InstanceStatus
		staleITS   bool
		wantPhase  opsv1alpha1.OpsPhase
		progress   string
	}{
		{name: "same target written at a later generation", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsSucceedPhase, progress: "2/2"},
		{name: "apps result waits", phase: appsv1.UpdatingComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps observation is stale", phase: appsv1.RunningComponentPhase, generation: 6, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps target is not up to date", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: false,
			instances: ready, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "apps result failed", phase: appsv1.FailedComponentPhase, generation: 7, upToDate: true,
			instances: ready, wantPhase: opsv1alpha1.OpsFailedPhase, progress: "2/2"},
		{name: "workload observation is stale", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready, staleITS: true, wantPhase: opsv1alpha1.OpsRunningPhase, progress: "2/2"},
		{name: "current progress is incomplete", phase: appsv1.RunningComponentPhase, generation: 7, upToDate: true,
			instances: ready[:1], wantPhase: opsv1alpha1.OpsRunningPhase, progress: "1/2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := verticalScalingTestCluster(component, tc.phase, tc.generation, tc.upToDate)
			ops := verticalScalingTestOps(request)
			ops.Status.ClusterGeneration = 6
			its := verticalScalingTestInstanceSet(component, tc.instances...)
			if tc.staleITS {
				its.Generation = 2
				its.Status.ObservedGeneration = 1
			}
			cli := verticalScalingTestClient(t, cluster, ops, its)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			if phase != tc.wantPhase || ops.Status.Progress != tc.progress {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%s", phase, ops.Status.Progress, tc.wantPhase, tc.progress)
			}
		})
	}
}

func TestVerticalScalingProgressUsesOnlySelectedTemplate(t *testing.T) {
	target := verticalScalingTestResources("2")
	other := verticalScalingTestResources("4")
	component := appsv1.ClusterComponentSpec{
		Name: "db", Replicas: 2,
		Instances: []appsv1.InstanceTemplate{
			{Name: "selected", Replicas: ptr.To[int32](1), Resources: &target},
			{Name: "other", Replicas: ptr.To[int32](1), Resources: &other},
		},
	}
	request := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
		Instances: []opsv1alpha1.InstanceResourceTemplate{{Name: "selected", ResourceRequirements: target}}}
	cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
	ops := verticalScalingTestOps(request)
	its := verticalScalingTestInstanceSet(component,
		verticalScalingTestInstance("demo-db-selected-0", "selected", true),
		verticalScalingTestInstance("demo-db-other-0", "other", false),
	)
	cli := verticalScalingTestClient(t, cluster, ops, its)

	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsSucceedPhase || ops.Status.Progress != "1/1" {
		t.Fatalf("got phase=%s progress=%s", phase, ops.Status.Progress)
	}
	details := ops.Status.Components["db"].ProgressDetails
	if len(details) != 1 || details[0].ObjectKey != "Pod/demo-db-selected-0" {
		t.Fatalf("unexpected progress details: %#v", details)
	}
}

func TestVerticalScalingDistinguishesZeroWorkFromMissingObservation(t *testing.T) {
	target := verticalScalingTestResources("2")
	for _, replicas := range []int32{0, 1} {
		t.Run(resource.NewQuantity(int64(replicas), resource.DecimalSI).String(), func(t *testing.T) {
			component := appsv1.ClusterComponentSpec{Name: "db", Replicas: replicas,
				Instances: []appsv1.InstanceTemplate{{Name: "selected", Replicas: ptr.To(replicas), Resources: &target}}}
			request := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"},
				Instances: []opsv1alpha1.InstanceResourceTemplate{{Name: "selected", ResourceRequirements: target}}}
			cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
			ops := verticalScalingTestOps(request)
			cli := verticalScalingTestClient(t, cluster, ops)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			wantPhase, wantProgress := opsv1alpha1.OpsRunningPhase, "0/1"
			if replicas == 0 {
				wantPhase, wantProgress = opsv1alpha1.OpsSucceedPhase, "0/0"
			}
			if phase != wantPhase || ops.Status.Progress != wantProgress {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%s", phase, ops.Status.Progress, wantPhase, wantProgress)
			}
		})
	}
}

func TestVerticalScalingCancellationUsesLastConfigurationAndRefreshesProgress(t *testing.T) {
	target := verticalScalingTestResources("2")
	previous := verticalScalingTestResources("1")
	component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1, Resources: previous}
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target,
	}
	cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
	ops := verticalScalingTestOps(request)
	ops.Spec.Cancel = true
	ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
	ops.Status.LastConfiguration.Components = map[string]opsv1alpha1.LastComponentConfiguration{
		"db": {ResourceRequirements: previous},
	}
	instance := verticalScalingTestInstance("demo-db-0", "", false)
	instance.Failed = true
	its := verticalScalingTestInstanceSet(component, instance)
	cli := verticalScalingTestClient(t, cluster, ops, its)
	handler := verticalScalingHandler{}
	reconcile := func(wantPhase opsv1alpha1.OpsPhase, wantProgress opsv1alpha1.ProgressStatus) {
		t.Helper()
		phase, _, err := handler.ReconcileAction(
			intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
		if err != nil {
			t.Fatal(err)
		}
		detail := ops.Status.Components["db"].ProgressDetails[0]
		if phase != wantPhase || detail.Status != wantProgress {
			t.Fatalf("got phase=%s detail=%s, want phase=%s detail=%s", phase, detail.Status, wantPhase, wantProgress)
		}
		if wantProgress == opsv1alpha1.ProcessingProgressStatus && !detail.EndTime.IsZero() {
			t.Fatalf("processing detail retained terminal end time: %#v", detail)
		}
	}

	reconcile(opsv1alpha1.OpsRunningPhase, opsv1alpha1.FailedProgressStatus)
	its.Status.InstanceStatus[0].Failed = false
	its.Status.InstanceStatus[0].UpToDate = false
	if err := cli.Status().Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	reconcile(opsv1alpha1.OpsRunningPhase, opsv1alpha1.ProcessingProgressStatus)
	its.Status.InstanceStatus[0].UpToDate = true
	its.Status.InstanceStatus[0].Ready = true
	its.Status.InstanceStatus[0].Available = true
	if err := cli.Status().Update(context.Background(), its); err != nil {
		t.Fatal(err)
	}
	reconcile(opsv1alpha1.OpsSucceedPhase, opsv1alpha1.SucceedProgressStatus)
	if message := ops.Status.Components["db"].ProgressDetails[0].Message; message == "" ||
		!containsAll(message, "rollback", "demo-db-0") {
		t.Fatalf("unexpected cancellation progress message %q", message)
	}
}

func TestVerticalScalingAbortsWhenManagedTargetIsOverwritten(t *testing.T) {
	target := verticalScalingTestResources("2")
	component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1, Resources: verticalScalingTestResources("3")}
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target,
	}
	cluster := verticalScalingTestCluster(component, appsv1.RunningComponentPhase, 7, true)
	ops := verticalScalingTestOps(request)
	cli := verticalScalingTestClient(t, cluster, ops)
	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsAbortedPhase {
		t.Fatalf("phase=%s, want %s", phase, opsv1alpha1.OpsAbortedPhase)
	}
}

func TestVerticalScalingShardingProgressWaitsForEveryCurrentShard(t *testing.T) {
	target := verticalScalingTestResources("2")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	for _, childCount := range []int{1, 2} {
		t.Run(resource.NewQuantity(int64(childCount), resource.DecimalSI).String(), func(t *testing.T) {
			cluster := &appsv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
				Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
					Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 1, Resources: target},
				}}},
				Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
					Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
				}}},
			}
			ops := verticalScalingTestOps(request)
			objects := []client.Object{cluster, ops}
			for i := 0; i < childCount; i++ {
				name := fmt.Sprintf("shard-%d", i)
				labels := constant.GetClusterLabels("demo", map[string]string{
					constant.KBAppShardingNameLabelKey: "shard",
				})
				labels[constant.KBAppComponentLabelKey] = name
				objects = append(objects,
					&appsv1.Component{ObjectMeta: metav1.ObjectMeta{
						Name: "demo-" + name, Namespace: "default", Labels: labels,
					}, Spec: appsv1.ComponentSpec{Replicas: 1}},
					&workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-" + name, Namespace: "default"},
						Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1)},
						Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
							verticalScalingTestInstance("demo-"+name+"-0", "", true),
						}}},
				)
			}
			cli := verticalScalingTestClient(t, objects...)
			phase, _, err := (verticalScalingHandler{}).ReconcileAction(
				intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
			if err != nil {
				t.Fatal(err)
			}
			wantPhase := opsv1alpha1.OpsRunningPhase
			if childCount == 2 {
				wantPhase = opsv1alpha1.OpsSucceedPhase
			}
			if phase != wantPhase || ops.Status.Progress != fmt.Sprintf("%d/2", childCount) {
				t.Fatalf("got phase=%s progress=%s, want phase=%s progress=%d/2", phase, ops.Status.Progress, wantPhase, childCount)
			}
		})
	}
}

func TestVerticalScalingShardingProgressDoesNotCrossCompensate(t *testing.T) {
	target := verticalScalingTestResources("2")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{Replicas: 1, Resources: target},
		}}},
		Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
		}}},
	}
	labels := constant.GetClusterLabels("demo", map[string]string{constant.KBAppShardingNameLabelKey: "shard"})
	labels[constant.KBAppComponentLabelKey] = "shard-0"
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Name: "demo-shard-0", Namespace: "default", Labels: labels,
	}, Spec: appsv1.ComponentSpec{Replicas: 2}}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-shard-0", Namespace: "default"},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To[int32](2)},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			verticalScalingTestInstance("demo-shard-0-0", "", true),
			verticalScalingTestInstance("demo-shard-0-1", "", true),
		}},
	}
	ops := verticalScalingTestOps(request)
	cli := verticalScalingTestClient(t, cluster, component, its, ops)
	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "2/3" {
		t.Fatalf("got phase=%s progress=%s, want phase=%s progress=2/3", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
}

func TestVerticalScalingShardingUsesPhysicalTemplateInheritanceAndCoverage(t *testing.T) {
	target := verticalScalingTestResources("2")
	override := verticalScalingTestResources("4")
	request := opsv1alpha1.VerticalScaling{
		ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "shard"}, ResourceRequirements: target,
	}
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec: appsv1.ClusterSpec{Shardings: []appsv1.ClusterSharding{{
			Name: "shard", Shards: 2, Template: appsv1.ClusterComponentSpec{
				Replicas: 1, Resources: target,
				Instances: []appsv1.InstanceTemplate{{Name: "reader", Replicas: ptr.To[int32](1), Resources: &override}},
			},
		}}},
		Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{"shard": {
			Phase: appsv1.RunningComponentPhase, ObservedGeneration: 7, UpToDate: true,
		}}},
	}
	labels := constant.GetClusterLabels("demo", map[string]string{constant.KBAppShardingNameLabelKey: "shard"})
	labels[constant.KBAppComponentLabelKey] = "shard-0"
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Name: "demo-shard-0", Namespace: "default", Labels: labels,
	}, Spec: appsv1.ComponentSpec{Replicas: 1, Instances: []appsv1.InstanceTemplate{{
		Name: "reader", Replicas: ptr.To[int32](1),
	}}}}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-shard-0", Namespace: "default"},
		Spec: workloads.InstanceSetSpec{Replicas: ptr.To[int32](1), Instances: []workloads.InstanceTemplate{{
			Name: "reader", Replicas: ptr.To[int32](1), Resources: nil,
		}}},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
			verticalScalingTestInstance("demo-shard-0-reader-0", "reader", true),
		}},
	}
	ops := verticalScalingTestOps(request)
	cli := verticalScalingTestClient(t, cluster, component, its, ops)
	phase, _, err := (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "1/1" {
		t.Fatalf("got phase=%s progress=%s, want phase=%s progress=1/1", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
	second := component.DeepCopy()
	second.Name = "demo-shard-1"
	second.ResourceVersion = ""
	second.Labels[constant.KBAppComponentLabelKey] = "shard-1"
	if err := cli.Create(context.Background(), second); err != nil {
		t.Fatal(err)
	}
	phase, _, err = (verticalScalingHandler{}).ReconcileAction(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, verticalScalingTestResourcesBundle(cluster, ops))
	if err != nil {
		t.Fatal(err)
	}
	if phase != opsv1alpha1.OpsRunningPhase || ops.Status.Progress != "1/2" {
		t.Fatalf("missing physical workload got phase=%s progress=%s, want phase=%s progress=1/2", phase, ops.Status.Progress, opsv1alpha1.OpsRunningPhase)
	}
}

func verticalScalingTestResources(cpu string) corev1.ResourceRequirements {
	return corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu)}}
}

func verticalScalingTestCluster(component appsv1.ClusterComponentSpec, phase appsv1.ComponentPhase,
	observedGeneration int64, upToDate bool) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", Generation: 7},
		Spec:       appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
		Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {
			Phase: phase, ObservedGeneration: observedGeneration, UpToDate: upToDate,
		}}},
	}
}

func verticalScalingTestOps(request opsv1alpha1.VerticalScaling) *opsv1alpha1.OpsRequest {
	return &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default"},
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			VerticalScalingList: []opsv1alpha1.VerticalScaling{request},
		}},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase: opsv1alpha1.OpsRunningPhase, ClusterGeneration: 7,
			LastConfiguration: opsv1alpha1.LastConfiguration{Components: map[string]opsv1alpha1.LastComponentConfiguration{}},
		},
	}
}

func verticalScalingTestInstance(name, template string, ready bool) workloads.InstanceStatus {
	return workloads.InstanceStatus{
		PodName: name, TemplateName: ptr.To(template), DesiredState: workloads.InstanceDesiredStateActive,
		CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: true, Ready: ready, Available: ready,
	}
}

func verticalScalingTestInstanceSet(component appsv1.ClusterComponentSpec,
	instances ...workloads.InstanceStatus) *workloads.InstanceSet {
	templates := make([]workloads.InstanceTemplate, 0, len(component.Instances))
	for i := range component.Instances {
		template := &component.Instances[i]
		templates = append(templates, workloads.InstanceTemplate{Name: template.Name, Replicas: template.Replicas})
	}
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To(component.Replicas), Instances: templates},
		Status:     workloads.InstanceSetStatus{InstanceStatus: instances},
	}
}

func verticalScalingTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{appsv1.AddToScheme, opsv1alpha1.AddToScheme, workloads.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&opsv1alpha1.OpsRequest{}, &workloads.InstanceSet{}).
		WithObjects(objects...).Build()
}

func verticalScalingTestResourcesBundle(cluster *appsv1.Cluster, ops *opsv1alpha1.OpsRequest) *OpsResource {
	return &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20)}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
