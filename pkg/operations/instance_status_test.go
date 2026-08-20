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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func templateName(name string) *string { return &name }

func TestInstanceStatusSelection(t *testing.T) {
	statuses := []workloads.InstanceStatus{
		{PodName: "demo-0", TemplateName: templateName(""), DesiredState: workloads.InstanceDesiredStateActive},
		{PodName: "demo-1", TemplateName: templateName("big"), DesiredState: workloads.InstanceDesiredStateActive},
		{PodName: "demo-2", DesiredState: workloads.InstanceDesiredStateOffline},
	}
	active, err := statusesToPodSet(statuses, workloads.InstanceDesiredStateActive, nil, true)
	if err != nil {
		t.Fatalf("select active assignments: %v", err)
	}
	if len(active) != 2 || active["demo-0"] != "" || active["demo-1"] != "big" {
		t.Fatalf("unexpected active assignments: %#v", active)
	}
	offline, err := statusesToPodSet(statuses, workloads.InstanceDesiredStateOffline, nil, false)
	if err != nil {
		t.Fatalf("select offline assignments: %v", err)
	}
	if len(offline) != 1 {
		t.Fatalf("unrelated offline instance with unknown template should still be selectable: %#v", offline)
	}
	if _, err := statusesToPodSet(statuses, workloads.InstanceDesiredStateOffline, nil, true); err == nil {
		t.Fatal("expected a relevant unknown offline template to wait")
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
}

func TestFlatHorizontalDiff(t *testing.T) {
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
	if !flatHorizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("expected explicit online/offline transition to match diff")
	}
	horizontalScaling.ScaleOut.OfflineInstancesToOnline = []string{"demo-3"}
	if flatHorizontalDiffMatchesOperation(horizontalScaling, created, deleted) {
		t.Fatal("stale allocation must not satisfy an explicit identity transition")
	}
	if flatHorizontalDiffMatchesOperation(opsv1alpha1.HorizontalScaling{ScaleOut: &opsv1alpha1.ScaleOut{}}, created, deleted) {
		t.Fatal("scale-out-only operation must not accept an unexpected deletion")
	}
}

func TestFlatInPlaceRebuildGetsTemplateFromInstanceStatus(t *testing.T) {
	const (
		namespace     = "default"
		clusterName   = "demo"
		componentName = "database"
		podName       = "identity-without-a-parseable-ordinal"
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
			PodName: podName, TemplateName: templateName("large"), DesiredState: workloads.InstanceDesiredStateActive,
		}}},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(its).Build()
	opsRes := &OpsResource{
		Cluster: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: clusterName},
			Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
				Name: componentName, FlatInstanceOrdinal: true,
			}}},
		},
		Runtimes: map[string]OpsRuntime{componentName: newOpsRuntime(context.Background(), cli, "")},
	}
	got, err := (rebuildInstanceOpsHandler{}).getInPlaceRebuildTemplateName(opsRes, componentName, componentName, podName)
	if err != nil {
		t.Fatalf("get template from InstanceStatus: %v", err)
	}
	if got != "large" {
		t.Fatalf("unexpected template: %q", got)
	}
}

func TestFlatFutureNameFeaturesFailBeforeMutation(t *testing.T) {
	const componentName = "database"
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "demo"},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name: componentName, Replicas: 1, FlatInstanceOrdinal: true,
		}}},
		Status: appsv1.ClusterStatus{Phase: appsv1.RunningClusterPhase},
	}

	rebuildOps := &opsv1alpha1.OpsRequest{Spec: opsv1alpha1.OpsRequestSpec{
		SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{RebuildFrom: []opsv1alpha1.RebuildInstance{{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName},
			Instances:    []opsv1alpha1.Instance{{Name: "old", TargetNodeName: "node-a"}},
		}}},
	}}
	err := (rebuildInstanceOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, nil,
		&OpsResource{Cluster: cluster.DeepCopy(), OpsRequest: rebuildOps})
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("expected flat non-in-place rebuild targetNodeName to fail fatally, got %v", err)
	}

	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := opsv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	replicas := int32(1)
	horizontalOps := &opsv1alpha1.OpsRequest{
		Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{
			ComponentOps: opsv1alpha1.ComponentOps{ComponentName: componentName},
			ScaleOut:     &opsv1alpha1.ScaleOut{FromBackup: &opsv1alpha1.FromBackup{Name: "backup"}},
		}}}},
		Status: opsv1alpha1.OpsRequestStatus{LastConfiguration: opsv1alpha1.LastConfiguration{
			Components: map[string]opsv1alpha1.LastComponentConfiguration{componentName: {Replicas: &replicas}},
		}},
	}
	clusterBefore := cluster.DeepCopy()
	err = (horizontalScalingOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
		&OpsResource{Cluster: cluster, OpsRequest: horizontalOps})
	if !intctrlutil.IsTargetError(err, intctrlutil.ErrorTypeFatal) {
		t.Fatalf("expected flat scale-out from backup to fail fatally, got %v", err)
	}
	if cluster.Spec.ComponentSpecs[0].Replicas != clusterBefore.Spec.ComponentSpecs[0].Replicas {
		t.Fatal("unsupported flat scale-out mutated the Cluster")
	}
}
