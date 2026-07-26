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

package cluster

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type clusterStatusCASClientCounters struct {
	creates         int
	patches         int
	statusPatches   int
	statusPatchType types.PatchType
	statusPatchData []byte
	statusPatchErr  error
}

func validClusterStatusCASPatch(resourceVersion string) []byte {
	return []byte(fmt.Sprintf(`[
		{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
		{"op":"test","path":"/metadata/resourceVersion","value":%q},
		{"op":"add","path":"/status/observedGeneration","value":7}
	]`, resourceVersion))
}

func newClusterStatusCASPlan(t *testing.T, counters *clusterStatusCASClientCounters,
	patches ...[]byte) (*clusterPlan, *appsv1.Cluster) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace:       "default",
		Name:            "cluster",
		UID:             "cluster-uid",
		ResourceVersion: "7",
	}}
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(cluster).
		WithObjects(cluster).
		Build()
	cli := interceptor.NewClient(baseClient, interceptor.Funcs{
		Create: func(ctx context.Context, cli client.WithWatch, obj client.Object,
			opts ...client.CreateOption) error {
			counters.creates++
			return cli.Create(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, cli client.WithWatch, obj client.Object,
			patch client.Patch, opts ...client.PatchOption) error {
			counters.patches++
			return cli.Patch(ctx, obj, patch, opts...)
		},
		SubResourcePatch: func(ctx context.Context, cli client.Client, subResourceName string,
			obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
			if subResourceName != "status" {
				return fmt.Errorf("unexpected subresource %q", subResourceName)
			}
			counters.statusPatches++
			counters.statusPatchType = patch.Type()
			data, err := patch.Data(obj)
			if err != nil {
				return err
			}
			counters.statusPatchData = append([]byte(nil), data...)
			return counters.statusPatchErr
		},
	})

	dag := graph.NewDAG()
	root := model.NewObjectVertex(cluster.DeepCopy(), cluster.DeepCopy(), model.ActionStatusPtr())
	dag.AddVertex(root)
	dag.AddConnect(root, model.NewObjectVertex(nil, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: cluster.Namespace, Name: "must-not-be-created"},
	}, model.ActionCreatePtr()))
	for _, patch := range patches {
		if err := addClusterStatusCASVertex(dag, cluster, patch); err != nil {
			t.Fatalf("add status CAS vertex: %v", err)
		}
	}

	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Logger:      logr.Discard(),
		Cluster:     cluster.DeepCopy(),
		OrigCluster: cluster.DeepCopy(),
	}
	builder := &clusterPlanBuilder{cli: cli, transCtx: transCtx}
	return &clusterPlan{
		dag:      dag,
		walkFunc: builder.defaultWalkFuncWithLogging,
		cli:      cli,
		transCtx: transCtx,
	}, cluster
}

func TestClusterPlanExecutesStatusCASExclusively(t *testing.T) {
	patch := validClusterStatusCASPatch("7")
	counters := &clusterStatusCASClientCounters{}
	plan, _ := newClusterStatusCASPlan(t, counters, patch)

	if err := plan.Execute(); err != nil {
		t.Fatalf("execute status CAS: %v", err)
	}
	if counters.statusPatches != 1 {
		t.Fatalf("status patch count = %d, want 1", counters.statusPatches)
	}
	if counters.statusPatchType != types.JSONPatchType {
		t.Fatalf("status patch type = %q, want %q", counters.statusPatchType, types.JSONPatchType)
	}
	if !bytes.Equal(counters.statusPatchData, patch) {
		t.Fatalf("status patch data = %s, want %s", counters.statusPatchData, patch)
	}
	if counters.creates != 0 || counters.patches != 0 {
		t.Fatalf("ordinary writes after exclusive CAS: creates=%d patches=%d, want 0/0",
			counters.creates, counters.patches)
	}
}

func TestClusterPlanStatusCASConflictDoesNotWriteFailureCondition(t *testing.T) {
	conflict := apierrors.NewConflict(schema.GroupResource{
		Group: appsv1.GroupVersion.Group, Resource: "clusters",
	}, "cluster", fmt.Errorf("resourceVersion changed"))
	counters := &clusterStatusCASClientCounters{statusPatchErr: conflict}
	plan, _ := newClusterStatusCASPlan(t, counters, validClusterStatusCASPatch("7"))

	err := plan.Execute()
	if !apierrors.IsConflict(err) {
		t.Fatalf("execute error = %v, want conflict", err)
	}
	if counters.statusPatches != 1 {
		t.Fatalf("status patch count = %d, want only the CAS attempt", counters.statusPatches)
	}
	if counters.creates != 0 || counters.patches != 0 {
		t.Fatalf("ordinary writes after CAS conflict: creates=%d patches=%d, want 0/0",
			counters.creates, counters.patches)
	}
}

func TestClusterPlanRejectsMultipleStatusCASVerticesBeforeWriting(t *testing.T) {
	counters := &clusterStatusCASClientCounters{}
	patch := validClusterStatusCASPatch("7")
	plan, _ := newClusterStatusCASPlan(t, counters, patch, patch)

	err := plan.Execute()
	if err == nil || !strings.Contains(err.Error(), "multiple exclusive Cluster status CAS intents") {
		t.Fatalf("execute error = %v, want duplicate CAS rejection", err)
	}
	if counters.statusPatches != 0 || counters.creates != 0 || counters.patches != 0 {
		t.Fatalf("writes before duplicate rejection: status=%d creates=%d patches=%d, want 0/0/0",
			counters.statusPatches, counters.creates, counters.patches)
	}
}

func TestClusterPlanRevalidatesStatusCASBeforeWriting(t *testing.T) {
	counters := &clusterStatusCASClientCounters{}
	plan, cluster := newClusterStatusCASPlan(t, counters)
	if !plan.dag.AddConnectRoot(&clusterStatusCASVertex{
		cluster: cluster.DeepCopy(),
		patch: []byte(`[
			{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
			{"op":"test","path":"/metadata/resourceVersion","value":"7"},
			{"op":"replace","path":"/spec/terminationPolicy","value":"Delete"}
		]`),
	}) {
		t.Fatal("add direct status CAS vertex")
	}

	if err := plan.Execute(); err == nil {
		t.Fatal("execute direct invalid status CAS succeeded, want rejection")
	}
	if counters.statusPatches != 0 || counters.creates != 0 || counters.patches != 0 {
		t.Fatalf("writes before direct status CAS rejection: status=%d creates=%d patches=%d, want 0/0/0",
			counters.statusPatches, counters.creates, counters.patches)
	}
}

func TestAddClusterStatusCASVertexRejectsInvalidPatchContract(t *testing.T) {
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace:       "default",
		Name:            "cluster",
		UID:             "cluster-uid",
		ResourceVersion: "7",
	}}
	tests := []struct {
		name  string
		patch []byte
	}{
		{
			name: "missing UID test",
			patch: []byte(`[
				{"op":"test","path":"/metadata/resourceVersion","value":"7"},
				{"op":"add","path":"/status/observedGeneration","value":7}
			]`),
		},
		{
			name: "missing resourceVersion test",
			patch: []byte(`[
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"add","path":"/status/observedGeneration","value":7}
			]`),
		},
		{
			name: "duplicate UID test",
			patch: []byte(`[
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"test","path":"/metadata/resourceVersion","value":"7"},
				{"op":"add","path":"/status/observedGeneration","value":7}
			]`),
		},
		{
			name:  "resourceVersion mismatch",
			patch: validClusterStatusCASPatch("8"),
		},
		{
			name: "spec mutation",
			patch: []byte(`[
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"test","path":"/metadata/resourceVersion","value":"7"},
				{"op":"replace","path":"/spec/terminationPolicy","value":"Delete"}
			]`),
		},
		{
			name: "test-only patch",
			patch: []byte(`[
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"test","path":"/metadata/resourceVersion","value":"7"}
			]`),
		},
		{
			name: "unsupported operation",
			patch: []byte(`[
				{"op":"test","path":"/metadata/uid","value":"cluster-uid"},
				{"op":"test","path":"/metadata/resourceVersion","value":"7"},
				{"op":"copy","from":"/status/phase","path":"/status/oldPhase"}
			]`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dag := graph.NewDAG()
			dag.AddVertex(model.NewObjectVertex(cluster.DeepCopy(), cluster.DeepCopy(), model.ActionStatusPtr()))
			if err := addClusterStatusCASVertex(dag, cluster, test.patch); err == nil {
				t.Fatalf("add Cluster status CAS succeeded, want rejection")
			}
			if len(dag.Vertices()) != 1 {
				t.Fatalf("DAG vertex count = %d, want only root", len(dag.Vertices()))
			}
		})
	}
}
