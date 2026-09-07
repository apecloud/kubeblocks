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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func templateName(name string) *string { return &name }

func TestVerticalScalingDefaultAndEmptySelection(t *testing.T) {
	for _, tc := range []struct {
		name                                string
		zeroTemplate, zeroComponent, cancel bool
	}{
		{name: "default"},
		{name: "zero-template", zeroTemplate: true},
		{name: "zero-component", zeroComponent: true},
		{name: "cancel-default", cancel: true},
		{name: "cancel-zero-template", zeroTemplate: true, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			target := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}}
			original := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}}
			comp := appsv1.ClusterComponentSpec{Name: "db", ComponentDef: "database", Replicas: 1, FlatInstanceOrdinal: true, Resources: target}
			vs := opsv1alpha1.VerticalScaling{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, ResourceRequirements: target}
			last := opsv1alpha1.LastComponentConfiguration{ResourceRequirements: original}
			if tc.zeroTemplate {
				comp.Instances = []appsv1.InstanceTemplate{{Name: "unused", Replicas: pointer.Int32(0), Resources: &target}}
				vs.ResourceRequirements = corev1.ResourceRequirements{}
				vs.Instances = []opsv1alpha1.InstanceResourceTemplate{{Name: "unused", ResourceRequirements: target}}
				last.Instances = []appsv1.InstanceTemplate{{Name: "unused", Resources: &original}}
			}
			if tc.zeroComponent {
				comp.Replicas = 0
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{comp}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scale", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{Cancel: tc.cancel, SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VerticalScalingList: []opsv1alpha1.VerticalScaling{vs}}},
				Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.NewTime(time.Now().Add(-time.Minute)),
					LastConfiguration: opsv1alpha1.LastConfiguration{Components: map[string]opsv1alpha1.LastComponentConfiguration{"db": last}}}}
			if tc.cancel {
				ops.Status.Phase = opsv1alpha1.OpsCancellingPhase
				ops.Status.CancelTimestamp = ops.Status.StartTimestamp
			}
			objects := []client.Object{cluster, its, ops, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}}
			// These unrelated Pods must never be selected, including when the
			// authoritative selection is empty. Their resources already match.
			for _, name := range []string{"demo-db-42", "demo-db-7", "demo-db-8"} {
				objects = append(objects, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", CreationTimestamp: metav1.Now(), Labels: constant.GetCompLabels("demo", "db")},
					Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "db", Resources: target}}},
					Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}})
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its).WithObjects(objects...).Build()
			opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(30), Runtimes: map[string]OpsRuntime{"db": newOpsRuntime(ctx, cli, "")}}
			handler := verticalScalingHandler{}
			if tc.cancel {
				if err := handler.Cancel(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes); err != nil {
					t.Fatal(err)
				}
			}
			check := func(want opsv1alpha1.OpsPhase, progress string, participants int) {
				t.Helper()
				phase, _, err := handler.ReconcileAction(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes)
				if err != nil {
					t.Fatal(err)
				}
				if phase != want || ops.Status.Progress != progress {
					t.Fatalf("got %s %s, want %s %s", phase, ops.Status.Progress, want, progress)
				}
				details := ops.Status.Components["db"].ProgressDetails
				if len(details) != participants {
					t.Fatalf("unexpected participants: %#v", details)
				}
				if participants == 1 && details[0].ObjectKey != "Pod/demo-db-42" {
					t.Fatalf("unexpected participant: %s", details[0].ObjectKey)
				}
			}
			if !tc.zeroComponent {
				check(opsv1alpha1.OpsRunningPhase, "0/1", 0)
			}
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: "demo-db-7", DesiredState: workloads.InstanceDesiredStateReleased},
				{PodName: "demo-db-8", DesiredState: workloads.InstanceDesiredStateOffline},
			}
			if !tc.zeroComponent {
				its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{PodName: "demo-db-42", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive})
			}
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			if tc.zeroTemplate || tc.zeroComponent {
				check(opsv1alpha1.OpsSucceedPhase, "0/0", 0)
				return
			}
			if tc.cancel {
				check(opsv1alpha1.OpsRunningPhase, "0/1", 1)
				pod := &corev1.Pod{}
				if err := cli.Get(ctx, client.ObjectKey{Name: "demo-db-42", Namespace: "default"}, pod); err != nil {
					t.Fatal(err)
				}
				pod.Spec.Containers[0].Resources = original
				if err := cli.Update(ctx, pod); err != nil {
					t.Fatal(err)
				}
			}
			check(opsv1alpha1.OpsSucceedPhase, "1/1", 1)
		})
	}
}

func TestStoppedVolumeExpansionUsesRetainedPVCs(t *testing.T) {
	for _, tc := range []struct {
		name, podName, template string
		flat                    bool
		defaultReplicas         bool
	}{
		{name: "flat-default", podName: "demo-db-42", flat: true},
		{name: "flat-template", podName: "demo-db-42", template: "large", flat: true},
		{name: "nonflat-default", podName: "demo-db-0"},
		{name: "nonflat-template", podName: "demo-db-large-0", template: "large"},
		{name: "template-default-replicas", podName: "demo-db-42", template: "large", flat: true, defaultReplicas: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			component := appsv1.ClusterComponentSpec{Name: "db", Replicas: 1, FlatInstanceOrdinal: tc.flat,
				Stop: pointer.Bool(true), OfflineInstances: []string{"demo-db-8"}}
			if tc.template != "" {
				component.Instances = []appsv1.InstanceTemplate{{Name: tc.template, Replicas: pointer.Int32(1)}}
				if tc.defaultReplicas {
					component.Instances[0].Replicas = nil
				}
			}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec:   appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{component}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.StoppedComponentPhase}}}}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
				Spec: workloads.InstanceSetSpec{Stop: pointer.Bool(true), Replicas: pointer.Int32(1), OfflineInstances: component.OfflineInstances},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
					{PodName: tc.podName, TemplateName: templateName(tc.template), DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
					{PodName: "demo-db-8", DesiredState: workloads.InstanceDesiredStateOffline, CurrentState: workloads.InstanceCurrentStateAbsent},
					{PodName: "demo-db-9", TemplateName: templateName(tc.template), DesiredState: workloads.InstanceDesiredStateReleased, CurrentState: workloads.InstanceCurrentStateTerminating},
				}}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "expand", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VolumeExpansionList: []opsv1alpha1.VolumeExpansion{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, VolumeClaimTemplates: []opsv1alpha1.OpsRequestVolumeClaimTemplate{{Name: "data", Storage: resource.MustParse("2Gi")}},
				}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase, StartTimestamp: metav1.Now()}}
			objects := []client.Object{cluster, its, ops}
			for _, name := range []string{tc.podName, "demo-db-8", "demo-db-9"} {
				labels := constant.GetCompLabels("demo", "db")
				labels[constant.VolumeClaimTemplateNameLabelKey] = "data"
				objects = append(objects, &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-" + name, Namespace: "default", Labels: labels},
					Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}},
					Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
						Conditions: []corev1.PersistentVolumeClaimCondition{{Type: corev1.PersistentVolumeClaimResizing, Status: corev1.ConditionTrue}}}})
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, &corev1.PersistentVolumeClaim{}).WithObjects(objects...).Build()
			opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(20), Runtimes: map[string]OpsRuntime{"db": newOpsRuntime(ctx, cli, "")}}
			check := func(want opsv1alpha1.OpsPhase, progress string) {
				t.Helper()
				phase, _, err := (volumeExpansionOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes)
				if err != nil {
					t.Fatal(err)
				}
				if phase != want || ops.Status.Progress != progress {
					t.Fatalf("got %s %s, want %s %s", phase, ops.Status.Progress, want, progress)
				}
				details := ops.Status.Components["db"].ProgressDetails
				if len(details) != 1 || details[0].ObjectKey != "PVC/data-"+tc.podName {
					t.Fatalf("unexpected participants: %#v", details)
				}
			}
			check(opsv1alpha1.OpsRunningPhase, "0/1")
			pvc := &corev1.PersistentVolumeClaim{}
			if err := cli.Get(ctx, client.ObjectKey{Namespace: "default", Name: "data-" + tc.podName}, pvc); err != nil {
				t.Fatal(err)
			}
			pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("2Gi")
			pvc.Status.Conditions = nil
			if err := cli.Status().Update(ctx, pvc); err != nil {
				t.Fatal(err)
			}
			check(opsv1alpha1.OpsSucceedPhase, "1/1")
		})
	}
}

// Exercise the real Operations runtime and handlers against explicit API objects.
// The instance names intentionally carry no template name or contiguous ordinal.
func TestScalingUsesAssignedInstancesAndActualObjects(t *testing.T) {
	ctx := context.Background()
	for _, operation := range []string{"vertical", "volume"} {
		t.Run(operation, func(t *testing.T) {
			scheme := runtime.NewScheme()
			for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, appsv1.AddToScheme, workloads.AddToScheme, opsv1alpha1.AddToScheme} {
				if err := add(scheme); err != nil {
					t.Fatal(err)
				}
			}
			resources := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}}
			cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
				Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{Name: "db", ComponentDef: "database", Replicas: 3, FlatInstanceOrdinal: true,
					Instances: []appsv1.InstanceTemplate{{Name: "large", Replicas: pointer.Int32(2), Resources: &resources}}}}},
				Status: appsv1.ClusterStatus{Components: map[string]appsv1.ClusterComponentStatus{"db": {Phase: appsv1.RunningComponentPhase}}}}
			its := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{Name: "demo-db", Namespace: "default"},
				Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{
					{PodName: "demo-db-7", TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: true},
					{PodName: "demo-db-42", TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent, UpToDate: true},
					{PodName: "demo-db-3", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
					{PodName: "demo-db-8", DesiredState: workloads.InstanceDesiredStateOffline},
					{PodName: "demo-db-9", TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateReleased},
				}}}
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "scaling", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{VerticalScalingList: []opsv1alpha1.VerticalScaling{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}, Instances: []opsv1alpha1.InstanceResourceTemplate{{Name: "large", ResourceRequirements: resources}},
				}}}}, Status: opsv1alpha1.OpsRequestStatus{Phase: opsv1alpha1.OpsRunningPhase}}
			objects := []client.Object{cluster, its, ops, &appsv1.ComponentDefinition{ObjectMeta: metav1.ObjectMeta{Name: "database"}}}
			for _, name := range []string{"demo-db-7", "demo-db-42", "demo-db-3", "demo-db-9"} {
				objects = append(objects, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: constant.GetCompLabels("demo", "db")},
					Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "db"}}, Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-" + name}}}}},
					Status: corev1.PodStatus{Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}}},
					&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-" + name, Namespace: "default", Labels: constant.GetCompLabels("demo", "db")},
						Spec:   corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}}},
						Status: corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound, Capacity: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}}})
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(ops, its, &corev1.PersistentVolumeClaim{}).WithObjects(objects...).Build()
			opsRes := &OpsResource{Cluster: cluster, OpsRequest: ops, Recorder: record.NewFakeRecorder(50), Runtimes: map[string]OpsRuntime{"db": newOpsRuntime(ctx, cli, "")}}
			compStatus := &opsv1alpha1.OpsRequestComponentStatus{}
			check := func(wantCompleted int) {
				t.Helper()
				if operation == "vertical" {
					phase, _, err := (verticalScalingHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes)
					if err != nil {
						t.Fatal(err)
					}
					wantPhase := opsv1alpha1.OpsRunningPhase
					if wantCompleted == 2 {
						wantPhase = opsv1alpha1.OpsSucceedPhase
					}
					if phase != wantPhase {
						t.Fatalf("phase=%s, want %s", phase, wantPhase)
					}
					status := ops.Status.Components["db"]
					compStatus = &status
				} else {
					succeeded, completed, err := (volumeExpansionOpsHandler{}).handleVCTExpansionProgress(intctrlutil.RequestCtx{Ctx: ctx}, cli, opsRes, compStatus, resource.MustParse("2Gi"), volumeExpansionHelper{
						compOps: opsv1alpha1.VolumeExpansion{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "db"}}, fullComponentName: "db", expectCount: 2, templateName: "large", vctName: "data"})
					if err != nil {
						t.Fatal(err)
					}
					if succeeded != wantCompleted || completed != wantCompleted {
						t.Fatalf("progress=%d/%d, want %d", succeeded, completed, wantCompleted)
					}
				}
				for _, detail := range compStatus.ProgressDetails {
					if detail.ObjectKey != "Pod/demo-db-7" && detail.ObjectKey != "Pod/demo-db-42" && detail.ObjectKey != "PVC/data-demo-db-7" && detail.ObjectKey != "PVC/data-demo-db-42" {
						t.Fatalf("unexpected participant: %s", detail.ObjectKey)
					}
				}
			}
			published := its.Status.InstanceStatus
			its.Status.InstanceStatus = nil
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			check(0) // Existing Pods do not replace an unpublished allocation.
			its.Status.InstanceStatus = published
			if err := cli.Status().Update(ctx, its); err != nil {
				t.Fatal(err)
			}
			check(0) // UpToDate alone must not bypass actual Pod/PVC checks.
			for i, name := range []string{"demo-db-7", "demo-db-42"} {
				key := client.ObjectKey{Namespace: "default", Name: name}
				if operation == "vertical" {
					pod := &corev1.Pod{}
					if err := cli.Get(ctx, key, pod); err != nil {
						t.Fatal(err)
					}
					pod.Spec.Containers[0].Resources = resources
					if err := cli.Update(ctx, pod); err != nil {
						t.Fatal(err)
					}
				} else {
					key.Name = "data-" + name
					pvc := &corev1.PersistentVolumeClaim{}
					if err := cli.Get(ctx, key, pvc); err != nil {
						t.Fatal(err)
					}
					pvc.Status.Capacity[corev1.ResourceStorage] = resource.MustParse("2Gi")
					if err := cli.Status().Update(ctx, pvc); err != nil {
						t.Fatal(err)
					}
				}
				check(i + 1)
			}
		})
	}
}

func TestInstanceStatusSelection(t *testing.T) {
	statuses := []workloads.InstanceStatus{
		{PodName: "demo-0", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
		{PodName: "demo-1", TemplateName: templateName("big"), DesiredState: workloads.InstanceDesiredStateActive},
		{PodName: "demo-2", DesiredState: workloads.InstanceDesiredStateOffline},
	}
	active, err := activeInstanceTemplates(statuses)
	if err != nil {
		t.Fatalf("select active assignments: %v", err)
	}
	if len(active) != 2 || active["demo-0"] != "" || active["demo-1"] != "big" {
		t.Fatalf("unexpected active assignments: %#v", active)
	}
	statuses[0].TemplateName = nil
	if _, err := activeInstanceTemplates(statuses); !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeNeedWaiting) {
		t.Fatalf("expected an active instance with unknown template to wait, got %v", err)
	}
	for _, invalid := range [][]workloads.InstanceStatus{
		{{PodName: ""}},
		{{PodName: "duplicate"}, {PodName: "duplicate"}},
	} {
		if _, err := activeInstanceTemplates(invalid); err == nil {
			t.Fatalf("invalid identities must not be silently accepted: %#v", invalid)
		}
	}
}

func TestActiveAssignmentsMatchComponent(t *testing.T) {
	component := &appsv1.ClusterComponentSpec{
		Replicas:  3,
		Instances: []appsv1.InstanceTemplate{{Name: "big", Replicas: pointer.Int32(1)}},
	}
	matching := map[string]string{"demo-0": "", "demo-1": "", "demo-2": "big"}
	if !assignmentsMatchComponent(matching, component) {
		t.Fatal("expected allocation to match component")
	}
	if assignmentsMatchComponent(map[string]string{"demo-0": "", "demo-1": "big", "demo-2": "big"}, component) {
		t.Fatal("template distribution mismatch must not be accepted")
	}
	component = &appsv1.ClusterComponentSpec{
		Replicas:  2,
		Instances: []appsv1.InstanceTemplate{{Name: "big", Replicas: pointer.Int32(0)}},
	}
	if !assignmentsMatchComponent(map[string]string{"demo-0": "", "demo-1": ""}, component) {
		t.Fatal("zero-replica templates must not require an allocation")
	}
}

func TestVolumeExpansionWaitsWithoutReportingSuccessForIncompleteAllocation(t *testing.T) {
	const (
		namespace     = "default"
		clusterName   = "demo"
		componentName = "database"
	)
	scheme := runtime.NewScheme()
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      constant.GenerateClusterComponentName(clusterName, componentName),
		},
		Status: workloads.InstanceSetStatus{InstanceStatus: []workloads.InstanceStatus{{
			PodName: "allocated-identity", TemplateName: templateName(""),
			DesiredState: workloads.InstanceDesiredStateActive,
		}}},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its).Build()
	opsRes := &OpsResource{
		Cluster:    &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName}},
		OpsRequest: &opsv1alpha1.OpsRequest{},
		Runtimes:   map[string]OpsRuntime{componentName: newOpsRuntime(context.Background(), cli, "")},
	}
	compStatus := &opsv1alpha1.OpsRequestComponentStatus{}
	succeeded, completed, err := (volumeExpansionOpsHandler{}).handleVCTExpansionProgress(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, compStatus,
		resource.MustParse("2Gi"), volumeExpansionHelper{
			compOps:           opsv1alpha1.VolumeExpansion{ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName}},
			fullComponentName: componentName,
			expectCount:       2,
			vctName:           "data",
		})
	if err != nil {
		t.Fatalf("wait for allocation: %v", err)
	}
	if succeeded != 0 || completed != 0 {
		t.Fatalf("incomplete allocation must not report progress, got succeeded=%d completed=%d", succeeded, completed)
	}
}
