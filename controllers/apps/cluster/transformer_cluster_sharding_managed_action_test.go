/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project.

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

package cluster

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	opsutil "github.com/apecloud/kubeblocks/pkg/operations/util"
)

type managedShardAddReadErrorClient struct {
	client.Client
	objectType reflect.Type
	err        error
}

func (c *managedShardAddReadErrorClient) Get(ctx context.Context, key client.ObjectKey,
	obj client.Object, opts ...client.GetOption) error {
	if reflect.TypeOf(obj) == c.objectType {
		return c.err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

const (
	managedShardAddTestNamespace = "default"
	managedShardAddTestCluster   = "redis"
	managedShardAddTestSharding  = "redis-cluster"
	managedShardAddTestMember    = "redis-redis-cluster-abcde"
	managedShardAddTestOpsDef    = "redis-shard-add"
)

func managedShardAddTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		appsv1.AddToScheme,
		opsv1alpha1.AddToScheme,
		batchv1.AddToScheme,
		corev1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
		model.AddScheme(add)
	}
	return scheme
}

func managedShardAddTestClusterObject() *appsv1.Cluster {
	return &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ClusterKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  managedShardAddTestNamespace,
			Name:       managedShardAddTestCluster,
			UID:        types.UID("cluster-uid"),
			Generation: 2,
		},
		Spec: appsv1.ClusterSpec{
			Shardings: []appsv1.ClusterSharding{{
				Name:        managedShardAddTestSharding,
				ShardingDef: "redis-sharding",
				Shards:      4,
			}},
		},
		Status: appsv1.ClusterStatus{Shardings: map[string]appsv1.ClusterShardingStatus{}},
	}
}

func managedShardAddTestDefinition() *appsv1.ShardingDefinition {
	return &appsv1.ShardingDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-sharding"},
		Spec: appsv1.ShardingDefinitionSpec{
			LifecycleActions: &appsv1.ShardingLifecycleActions{
				ShardAdd: &appsv1.ShardingAction{OpsDefinitionName: managedShardAddTestOpsDef},
			},
		},
	}
}

func managedShardAddTestOpsDefinition() *opsv1alpha1.OpsDefinition {
	return &opsv1alpha1.OpsDefinition{
		TypeMeta: metav1.TypeMeta{APIVersion: opsv1alpha1.GroupVersion.String(), Kind: "OpsDefinition"},
		ObjectMeta: metav1.ObjectMeta{
			Name:       managedShardAddTestOpsDef,
			UID:        types.UID("opsdef-uid"),
			Generation: 3,
		},
		Spec: opsv1alpha1.OpsDefinitionSpec{
			Actions: []opsv1alpha1.OpsAction{{
				Name:          "rebalance",
				FailurePolicy: opsv1alpha1.FailurePolicyFail,
				Workload: &opsv1alpha1.OpsWorkloadAction{
					Type: opsv1alpha1.ManagedJobWorkload,
					PodSpec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers:    []corev1.Container{{Name: "worker", Image: "redis:7"}},
					},
				},
			}},
		},
		Status: opsv1alpha1.OpsDefinitionStatus{
			ObservedGeneration: 3,
			Phase:              opsv1alpha1.AvailablePhase,
		},
	}
}

func managedShardAddTestContext(t *testing.T, objects ...client.Object) (*clusterTransformContext, model.GraphClient, client.Client) {
	t.Helper()
	cluster := managedShardAddTestClusterObject()
	fakeClient := fake.NewClientBuilder().WithScheme(managedShardAddTestScheme(t)).WithObjects(objects...).Build()
	graphClient := model.NewGraphClient(fakeClient)
	ctx := &clusterTransformContext{
		Context:     context.Background(),
		Client:      graphClient,
		APIReader:   fakeClient,
		Cluster:     cluster,
		OrigCluster: cluster.DeepCopy(),
		shardings:   []*appsv1.ClusterSharding{&cluster.Spec.Shardings[0]},
		shardingDefs: map[string]*appsv1.ShardingDefinition{
			"redis-sharding": managedShardAddTestDefinition(),
		},
	}
	return ctx, graphClient, fakeClient
}

func managedShardAddTestDAG(graphClient model.GraphClient, cluster *appsv1.Cluster) *graph.DAG {
	dag := graph.NewDAG()
	graphClient.Root(dag, cluster.DeepCopy(), cluster, model.ActionStatusPtr())
	return dag
}

func TestManagedShardAddPersistsPlanBeforeCreatingMembers(t *testing.T) {
	ctx, graphClient, _ := managedShardAddTestContext(t)
	handler := &clusterShardingHandler{}
	proto := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: managedShardAddTestNamespace,
		Name:      managedShardAddTestMember,
	}}
	toCreate := sets.New[string](managedShardAddTestMember)

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{proto.Name: proto}, toCreate)
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("first reconcile handled=%v err=%v, want delayed managed plan", handled, err)
	}
	status := ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if status == nil || status.MembersDispatched || status.PlanHash == "" || status.Token == "" {
		t.Fatalf("first reconcile did not persist an undispatched exact plan: %#v", status)
	}
	if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
		t.Fatalf("first reconcile created %d Components before plan persistence", len(got))
	}

	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{proto.Name: proto}, toCreate)
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("second reconcile handled=%v err=%v, want delayed member create", handled, err)
	}
	status = ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if !status.MembersDispatched {
		t.Fatal("second reconcile did not durably record member dispatch")
	}
	created := graphClient.FindAll(dag, &appsv1.Component{})
	if len(created) != 1 || !graphClient.IsAction(dag, created[0], model.ActionCreatePtr()) {
		t.Fatalf("second reconcile Component vertices=%d, want exactly one create", len(created))
	}
	marker, err := decodeManagedShardAddMarker(created[0].GetAnnotations()[shardingAddShardKey])
	if err != nil {
		t.Fatalf("created Component marker is invalid: %v", err)
	}
	if !reflect.DeepEqual(marker.Members, []string{managedShardAddTestMember}) || marker.PlanHash != status.PlanHash {
		t.Fatalf("created Component marker=%#v, status=%#v", marker, status)
	}
}

func TestManagedShardAddMemberDispatchRecoversPartialCreate(t *testing.T) {
	const secondMember = "redis-redis-cluster-fghij"
	ctx, graphClient, fakeClient := managedShardAddTestContext(t)
	ctx.Cluster.Spec.Shardings[0].Shards = 5
	status, err := buildManagedShardAddPlan(ctx.Cluster, managedShardAddTestSharding, 5,
		[]string{managedShardAddTestMember, secondMember})
	if err != nil {
		t.Fatal(err)
	}
	status.OpsDefinitionName = managedShardAddTestOpsDef
	ctx.Cluster.Status.Shardings[managedShardAddTestSharding] = appsv1.ClusterShardingStatus{ShardAdd: status}
	marker, err := encodeManagedShardAddMarker(markerForManagedShardAdd(ctx.Cluster, managedShardAddTestSharding, status))
	if err != nil {
		t.Fatal(err)
	}
	existing := &appsv1.Component{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   managedShardAddTestNamespace,
			Name:        managedShardAddTestMember,
			UID:         types.UID("partially-created-uid"),
			Annotations: map[string]string{shardingAddShardKey: marker},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(ctx.Cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		},
	}
	if err := fakeClient.Create(context.Background(), existing); err != nil {
		t.Fatal(err)
	}
	missing := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: managedShardAddTestNamespace,
		Name:      secondMember,
	}}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := (&clusterShardingHandler{}).handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{existing.Name: existing},
		map[string]*appsv1.Component{existing.Name: existing.DeepCopy(), missing.Name: missing},
		sets.New[string](missing.Name))
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("partial recovery handled=%v err=%v, want delayed create", handled, err)
	}
	if !status.MembersDispatched || status.Phase != appsv1.LifecycleActionRunning {
		t.Fatalf("partial recovery did not close dispatch state: %#v", status)
	}
	created := graphClient.FindAll(dag, &appsv1.Component{})
	if len(created) != 1 || created[0].GetName() != secondMember ||
		!graphClient.IsAction(dag, created[0], model.ActionCreatePtr()) {
		t.Fatalf("partial recovery creates=%#v, want only %s", created, secondMember)
	}
}

func managedShardAddReadyContext(t *testing.T) (*clusterTransformContext, model.GraphClient, client.Client,
	*clusterShardingHandler, *appsv1.ShardingActionStatus) {
	t.Helper()
	cluster := managedShardAddTestClusterObject()
	status, err := buildManagedShardAddPlan(cluster, managedShardAddTestSharding, 4, []string{managedShardAddTestMember})
	if err != nil {
		t.Fatal(err)
	}
	status.MembersDispatched = true
	status.OpsDefinitionName = managedShardAddTestOpsDef
	marker, err := encodeManagedShardAddMarker(markerForManagedShardAdd(cluster, managedShardAddTestSharding, status))
	if err != nil {
		t.Fatal(err)
	}
	member := &appsv1.Component{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  managedShardAddTestNamespace,
			Name:       managedShardAddTestMember,
			UID:        types.UID("component-uid"),
			Generation: 1,
			Annotations: map[string]string{
				shardingAddShardKey: marker,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		},
		Status: appsv1.ComponentStatus{Phase: appsv1.RunningComponentPhase, ObservedGeneration: 1},
	}
	ctx, graphClient, fakeClient := managedShardAddTestContext(t, member, managedShardAddTestOpsDefinition())
	ctx.Cluster.Status.Shardings[managedShardAddTestSharding] = appsv1.ClusterShardingStatus{ShardAdd: status}
	return ctx, graphClient, fakeClient, &clusterShardingHandler{}, status
}

func TestManagedShardAddActivePlanFreezesLaterTopologyChanges(t *testing.T) {
	tests := []struct {
		name       string
		toCreate   sets.Set[string]
		protoComps map[string]*appsv1.Component
	}{
		{
			name:     "later scale-in",
			toCreate: sets.New[string](),
		},
		{
			name:     "later scale-out",
			toCreate: sets.New[string]("redis-redis-cluster-later"),
			protoComps: map[string]*appsv1.Component{
				"redis-redis-cluster-later": {ObjectMeta: metav1.ObjectMeta{
					Namespace: managedShardAddTestNamespace,
					Name:      "redis-redis-cluster-later",
				}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, graphClient, _, handler, status := managedShardAddReadyContext(t)
			dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
			handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
				map[string]*appsv1.Component{}, test.protoComps, test.toCreate)
			if !handled || !intctrlutil.IsDelayedRequeueError(err) {
				t.Fatalf("handled=%v err=%v, want the active plan to retain topology ownership", handled, err)
			}
			if status.TargetShardCount != 4 || len(status.Members) != 1 ||
				status.Members[0].Name != managedShardAddTestMember {
				t.Fatalf("active plan changed with a later desired topology: %#v", status)
			}
			if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
				t.Fatalf("active plan scheduled %d Component writes for a later desired topology", len(got))
			}
		})
	}
}

func TestManagedShardAddStartsOneCoalescedPlanAfterCleanup(t *testing.T) {
	const (
		firstNewMember  = "redis-redis-cluster-later-a"
		secondNewMember = "redis-redis-cluster-later-b"
	)
	ctx, graphClient, fakeClient, handler, status := managedShardAddReadyContext(t)
	status.Phase = appsv1.LifecycleActionSucceeded
	status.CleanupStarted = true
	status.MarkersCleaned = true
	status.CompletionTime = ptr.To(metav1.Now())
	ctx.Cluster.Generation = 4
	ctx.Cluster.Spec.Shardings[0].Shards = 6

	liveMember := &appsv1.Component{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{
		Namespace: managedShardAddTestNamespace,
		Name:      managedShardAddTestMember,
	}, liveMember); err != nil {
		t.Fatal(err)
	}
	delete(liveMember.Annotations, shardingAddShardKey)
	if err := fakeClient.Update(context.Background(), liveMember); err != nil {
		t.Fatal(err)
	}

	protoComps := map[string]*appsv1.Component{
		firstNewMember:  {ObjectMeta: metav1.ObjectMeta{Namespace: managedShardAddTestNamespace, Name: firstNewMember}},
		secondNewMember: {ObjectMeta: metav1.ObjectMeta{Namespace: managedShardAddTestNamespace, Name: secondNewMember}},
	}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, protoComps, sets.New[string](secondNewMember, firstNewMember))
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("handled=%v err=%v, want one persisted follow-up plan", handled, err)
	}

	next := ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if next == status || next.ClusterGeneration != 4 || next.TargetShardCount != 6 ||
		next.MembersDispatched || !reflect.DeepEqual([]string{next.Members[0].Name, next.Members[1].Name},
		[]string{firstNewMember, secondNewMember}) {
		t.Fatalf("follow-up plan did not coalesce the latest desired topology: %#v", next)
	}
	if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
		t.Fatalf("follow-up plan created %d Components before its status was persisted", len(got))
	}
}

func TestManagedShardAddDoesNotFallBackAfterManagedActionRemoval(t *testing.T) {
	ctx, graphClient, _, handler, status := managedShardAddReadyContext(t)
	status.Phase = appsv1.LifecycleActionSucceeded
	status.CleanupStarted = true
	status.MarkersCleaned = true
	status.CompletionTime = ptr.To(metav1.Now())
	ctx.shardingDefs["redis-sharding"].Spec.LifecycleActions.ShardAdd = nil

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]("redis-redis-cluster-next"))
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want fail-loud managed ownership", handled, err)
	}
	next := ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if next == status || next.Phase != appsv1.LifecycleActionFailed || next.Reason != "OpsDefinitionChanged" ||
		next.MarkersCleaned || len(next.Members) != 1 || next.Members[0].Name != "redis-redis-cluster-next" {
		t.Fatalf("removed managed action did not persist one failed follow-up plan: %#v", next)
	}
	if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
		t.Fatalf("removed managed action scheduled %d legacy Component writes", len(got))
	}
	projected := (&clusterComponentStatusTransformer{}).buildClusterShardingStatus(ctx,
		managedShardAddTestSharding, nil)
	if projected.Phase != appsv1.FailedComponentPhase || projected.UpToDate {
		t.Fatalf("failed follow-up plan was not projected as topology-blocking: %#v", projected)
	}
}

func TestManagedShardAddLegacyMarkerFailsLoudWithoutRewriting(t *testing.T) {
	cluster := managedShardAddTestClusterObject()
	member := &appsv1.Component{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: appsv1.ComponentKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   managedShardAddTestNamespace,
			Name:        managedShardAddTestMember,
			UID:         types.UID("legacy-component-uid"),
			Annotations: map[string]string{shardingAddShardKey: "2026-07-16T12:00:00Z"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
			},
		},
	}
	ctx, graphClient, _ := managedShardAddTestContext(t, member)
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := (&clusterShardingHandler{}).handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{member.Name: member}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil {
		t.Fatalf("handled=%v err=%v, want a durable fail-loud state", handled, err)
	}
	status := ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if status == nil || status.Phase != appsv1.LifecycleActionFailed || status.Reason != "MarkerRecoveryFailed" {
		t.Fatalf("legacy marker did not become an explicit recovery failure: %#v", status)
	}
	if member.Annotations[shardingAddShardKey] != "2026-07-16T12:00:00Z" {
		t.Fatalf("legacy marker was rewritten: %#v", member.Annotations)
	}
	if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
		t.Fatalf("legacy marker failure scheduled %d Component writes", len(got))
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("legacy marker failure scheduled %d OpsRequest writes", len(got))
	}
}

func TestManagedShardAddMarkerRejectsTrailingData(t *testing.T) {
	encoded, err := encodeManagedShardAddMarker(managedShardAddMarker{
		Version:           managedShardAddMarkerVersion,
		ClusterUID:        "cluster-uid",
		ClusterGeneration: 3,
		ShardingName:      managedShardAddTestSharding,
		Token:             "token",
		PlanHash:          "plan-hash",
		TargetShardCount:  4,
		Members:           []string{managedShardAddTestMember},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeManagedShardAddMarker(encoded + `{"unexpected":true}`); err == nil {
		t.Fatal("marker with a second JSON value was accepted")
	}
}

func advanceManagedShardAddToOpsRequestPlan(t *testing.T) (*clusterTransformContext, model.GraphClient,
	client.Client, *clusterShardingHandler, *appsv1.ShardingActionStatus, *opsv1alpha1.OpsRequest) {
	t.Helper()
	ctx, graphClient, fakeClient, handler, status := managedShardAddReadyContext(t)
	for step := 0; step < 2; step++ {
		dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
		handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
			map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
		if !handled || !intctrlutil.IsDelayedRequeueError(err) {
			t.Fatalf("advance step %d handled=%v err=%v, want delayed", step, handled, err)
		}
	}
	if status.MemberSnapshotHash == "" || status.OpsDefinitionUID == "" ||
		status.OpsRequestName == "" || status.OpsRequestSpecHash == "" {
		t.Fatalf("execution/OpsRequest plan is incomplete: %#v", status)
	}
	expected, _, err := buildManagedShardAddOpsRequest(ctx.Cluster, managedShardAddTestSharding, status)
	if err != nil {
		t.Fatal(err)
	}
	return ctx, graphClient, fakeClient, handler, status, expected
}

func TestManagedShardAddOpsRequestUnboundNotFoundCreatesOnce(t *testing.T) {
	ctx, graphClient, _, handler, _, expected := advanceManagedShardAddToOpsRequestPlan(t)
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("handled=%v err=%v, want delayed create", handled, err)
	}
	created := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{})
	if len(created) != 1 || created[0].GetName() != expected.Name ||
		!graphClient.IsAction(dag, created[0], model.ActionCreatePtr()) {
		t.Fatalf("OpsRequest creates=%d, want exactly one deterministic create", len(created))
	}
}

func TestManagedShardAddReadErrorDoesNotBecomeTerminalIdentityFailure(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status := managedShardAddReadyContext(t)
	wantErr := errors.New("temporary API transport failure")
	ctx.APIReader = &managedShardAddReadErrorClient{
		Client:     fakeClient,
		objectType: reflect.TypeOf(&appsv1.Component{}),
		err:        wantErr,
	}

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !errors.Is(err, wantErr) {
		t.Fatalf("handled=%v err=%v, want retryable read error", handled, err)
	}
	if status.Phase == appsv1.LifecycleActionFailed {
		t.Fatalf("retryable read error became terminal: %#v", status)
	}
}

func TestManagedShardAddRejectsTerminatingExactIdentities(t *testing.T) {
	cluster := managedShardAddTestClusterObject()
	status, err := buildManagedShardAddPlan(cluster, managedShardAddTestSharding, 4,
		[]string{managedShardAddTestMember})
	if err != nil {
		t.Fatal(err)
	}
	status.OpsDefinitionName = managedShardAddTestOpsDef
	marker, err := encodeManagedShardAddMarker(markerForManagedShardAdd(cluster, managedShardAddTestSharding, status))
	if err != nil {
		t.Fatal(err)
	}
	now := metav1.Now()
	member := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace:         managedShardAddTestNamespace,
		Name:              managedShardAddTestMember,
		UID:               types.UID("component-uid"),
		DeletionTimestamp: &now,
		Finalizers:        []string{"test-finalizer"},
		Annotations:       map[string]string{shardingAddShardKey: marker},
		OwnerReferences: []metav1.OwnerReference{
			*metav1.NewControllerRef(cluster, appsv1.GroupVersion.WithKind(appsv1.ClusterKind)),
		},
	}}
	if err := validateManagedShardAddMember(cluster, member, status.Members[0], marker); err == nil {
		t.Fatal("terminating exact Component was accepted")
	}

	status.MemberSnapshotHash = strings.Repeat("a", 64)
	status.OpsDefinitionUID = "opsdef-uid"
	status.OpsDefinitionGeneration = 3
	status.OpsDefinitionSpecHash = strings.Repeat("b", 64)
	expected, hash, err := buildManagedShardAddOpsRequest(cluster, managedShardAddTestSharding, status)
	if err != nil {
		t.Fatal(err)
	}
	status.OpsRequestSpecHash = hash
	liveOps := expected.DeepCopy()
	liveOps.UID = types.UID("opsrequest-uid")
	liveOps.DeletionTimestamp = &now
	liveOps.Finalizers = []string{"test-finalizer"}
	if err := validateManagedShardAddOpsRequest(cluster, status, expected, hash, liveOps); err == nil {
		t.Fatal("terminating exact OpsRequest was accepted")
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         managedShardAddTestNamespace,
			Name:              "terminating-managed-job",
			UID:               types.UID("managed-job-uid"),
			DeletionTimestamp: &now,
			Finalizers:        []string{"test-finalizer"},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(liveOps, opsv1alpha1.GroupVersion.WithKind("OpsRequest")),
			},
		},
	}
	jobHash, err := opsutil.ManagedJobSpecHash(job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	taskIndex := int32(0)
	task := &opsv1alpha1.ActionTask{
		ObjectKey:        fmt.Sprintf("%s/%s", constant.JobKind, job.Name),
		Namespace:        job.Namespace,
		TaskIndex:        &taskIndex,
		DispatchState:    opsv1alpha1.CreatedActionTaskDispatchState,
		WorkloadUID:      string(job.UID),
		WorkloadSpecHash: jobHash,
	}
	ctx, _, _ := managedShardAddTestContext(t, job)
	if _, err := validateManagedShardAddJob(ctx, status, liveOps, task); err == nil {
		t.Fatal("terminating exact managed Job was accepted")
	}
}

func TestManagedShardAddOpsRequestUnboundFoundBindsWithoutCreate(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, expected := advanceManagedShardAddToOpsRequestPlan(t)
	live := expected.DeepCopy()
	live.UID = types.UID("opsrequest-uid")
	if err := fakeClient.Create(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("handled=%v err=%v, want delayed bind", handled, err)
	}
	if status.OpsRequestUID != string(live.UID) {
		t.Fatalf("bound OpsRequest UID=%q, want %q", status.OpsRequestUID, live.UID)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("found-path scheduled %d OpsRequest writes, want zero", len(got))
	}
}

func TestManagedShardAddBoundOpsRequestNeverRebindsNewUID(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, expected := advanceManagedShardAddToOpsRequestPlan(t)
	live := expected.DeepCopy()
	live.UID = types.UID("opsrequest-uid-1")
	if err := fakeClient.Create(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	_, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !intctrlutil.IsDelayedRequeueError(err) || status.OpsRequestUID != string(live.UID) {
		t.Fatalf("initial bind err=%v status=%#v", err, status)
	}
	if err := fakeClient.Delete(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	replacement := expected.DeepCopy()
	replacement.UID = types.UID("opsrequest-uid-2")
	if err := fakeClient.Create(context.Background(), replacement); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil {
		t.Fatalf("replacement handled=%v err=%v", handled, err)
	}
	if status.Phase != appsv1.LifecycleActionFailed || status.OpsRequestUID != string(live.UID) {
		t.Fatalf("replacement rebound or did not fail closed: %#v", status)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("bound replacement path scheduled %d OpsRequest writes, want zero", len(got))
	}
}

func bindManagedShardAddTestOpsRequest(t *testing.T) (*clusterTransformContext, model.GraphClient,
	client.Client, *clusterShardingHandler, *appsv1.ShardingActionStatus, *opsv1alpha1.OpsRequest) {
	t.Helper()
	ctx, graphClient, fakeClient, handler, status, expected := advanceManagedShardAddToOpsRequestPlan(t)
	live := expected.DeepCopy()
	live.UID = types.UID("opsrequest-uid")
	live.Status.Phase = opsv1alpha1.OpsRunningPhase
	if err := fakeClient.Create(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.OpsRequestUID != string(live.UID) {
		t.Fatalf("bind handled=%v err=%v status=%#v", handled, err, status)
	}
	return ctx, graphClient, fakeClient, handler, status, live
}

func attachManagedShardAddTestJob(t *testing.T, fakeClient client.Client, live *opsv1alpha1.OpsRequest,
	phase opsv1alpha1.OpsPhase) *batchv1.Job {
	t.Helper()
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: batchv1.SchemeGroupVersion.String(), Kind: constant.JobKind},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: managedShardAddTestNamespace,
			Name:      "redis-shard-add-job",
			UID:       types.UID("managed-job-uid"),
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(live, opsv1alpha1.GroupVersion.WithKind("OpsRequest")),
			},
		},
		Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers:    []corev1.Container{{Name: "worker", Image: "redis:7"}},
		}}},
	}
	hash, err := opsutil.ManagedJobSpecHash(job.Spec)
	if err != nil {
		t.Fatal(err)
	}
	taskIndex := int32(0)
	live.Status.Phase = phase
	live.Status.Components = map[string]opsv1alpha1.OpsRequestComponentStatus{
		managedShardAddTestSharding: {
			ProgressDetails: []opsv1alpha1.ProgressStatusDetail{{
				ActionName: "rebalance",
				ActionTasks: []opsv1alpha1.ActionTask{{
					ObjectKey:        constant.JobKind + "/" + job.Name,
					Namespace:        job.Namespace,
					Status:           opsv1alpha1.ProcessingActionTaskStatus,
					TaskIndex:        &taskIndex,
					DispatchState:    opsv1alpha1.CreatedActionTaskDispatchState,
					WorkloadUID:      string(job.UID),
					WorkloadSpecHash: hash,
				}},
			}},
		},
	}
	if err := fakeClient.Update(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	if err := fakeClient.Create(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	return job
}

func TestManagedShardAddSuccessfulJobCleansExactMarkersBeforeClosing(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, live := bindManagedShardAddTestOpsRequest(t)
	job := attachManagedShardAddTestJob(t, fakeClient, live, opsv1alpha1.OpsRunningPhase)

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.JobUID != string(job.UID) {
		t.Fatalf("job bind handled=%v err=%v status=%#v", handled, err, status)
	}

	live.Status.Phase = opsv1alpha1.OpsSucceedPhase
	live.Status.Components[managedShardAddTestSharding].ProgressDetails[0].ActionTasks[0].Status =
		opsv1alpha1.SucceedActionTaskStatus
	if err := fakeClient.Update(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := fakeClient.Status().Update(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	observedOps := &opsv1alpha1.OpsRequest{}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(live), observedOps); err != nil {
		t.Fatal(err)
	}
	if got := observedOps.Status.Components[managedShardAddTestSharding].ProgressDetails[0].ActionTasks[0].Status; got != opsv1alpha1.SucceedActionTaskStatus {
		t.Fatalf("test setup task status=%q, want Succeed", got)
	}
	observedJob := &batchv1.Job{}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(job), observedJob); err != nil {
		t.Fatal(err)
	}
	if !managedShardAddJobSucceeded(observedJob) {
		t.Fatalf("test setup Job conditions=%#v, want Complete=True", observedJob.Status.Conditions)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("cleanup handled=%v err=%v status=%#v", handled, err, status)
	}
	if status.Phase != appsv1.LifecycleActionRunning || !status.CleanupStarted ||
		status.MarkersCleaned || status.Reason != "CleanupPending" || status.CompletionTime != nil {
		t.Fatalf("success did not persist a non-terminal cleanup gate: %#v", status)
	}
	if writes := graphClient.FindAll(dag, &appsv1.Component{}); len(writes) != 0 {
		t.Fatalf("success transition scheduled %d marker writes before terminal status persistence", len(writes))
	}

	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("cleanup handled=%v err=%v", handled, err)
	}
	patched := graphClient.FindAll(dag, &appsv1.Component{})
	if len(patched) != 1 || !graphClient.IsAction(dag, patched[0], model.ActionPatchPtr()) ||
		patched[0].GetAnnotations()[shardingAddShardKey] != "" {
		t.Fatalf("marker cleanup patches=%d objects=%#v", len(patched), patched)
	}

	member := &appsv1.Component{}
	memberKey := client.ObjectKey{Namespace: managedShardAddTestNamespace, Name: managedShardAddTestMember}
	if err := fakeClient.Get(context.Background(), memberKey, member); err != nil {
		t.Fatal(err)
	}
	delete(member.Annotations, shardingAddShardKey)
	if err := fakeClient.Update(context.Background(), member); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || !status.MarkersCleaned ||
		status.Phase != appsv1.LifecycleActionSucceeded || status.Reason != "Completed" || status.CompletionTime == nil {
		t.Fatalf("close handled=%v err=%v status=%#v", handled, err, status)
	}

	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if handled || err != nil {
		t.Fatalf("closed action handled=%v err=%v, want generic topology flow", handled, err)
	}
}

func TestManagedShardAddRejectsSuccessfulOpsRequestWithoutSuccessfulJob(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, live := bindManagedShardAddTestOpsRequest(t)
	job := attachManagedShardAddTestJob(t, fakeClient, live, opsv1alpha1.OpsRunningPhase)

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	_, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !intctrlutil.IsDelayedRequeueError(err) || status.JobUID != string(job.UID) {
		t.Fatalf("job bind err=%v status=%#v", err, status)
	}

	live.Status.Phase = opsv1alpha1.OpsSucceedPhase
	if err := fakeClient.Update(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil || status.Phase != appsv1.LifecycleActionFailed ||
		status.Reason != "ManagedJobStatusInvalid" || status.CleanupStarted {
		t.Fatalf("incoherent success handled=%v err=%v status=%#v", handled, err, status)
	}
	if writes := graphClient.FindAll(dag, &appsv1.Component{}); len(writes) != 0 {
		t.Fatalf("incoherent success scheduled %d marker writes", len(writes))
	}
}

func TestManagedShardAddUnsuccessfulOpsRequestFailsWithoutMarkerCleanup(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, live := bindManagedShardAddTestOpsRequest(t)
	live.Status.Phase = opsv1alpha1.OpsFailedPhase
	if err := fakeClient.Update(context.Background(), live); err != nil {
		t.Fatal(err)
	}

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil || status.Phase != appsv1.LifecycleActionFailed ||
		status.Reason != "OpsRequestUnsuccessful" || status.MarkersCleaned {
		t.Fatalf("failed OpsRequest handled=%v err=%v status=%#v", handled, err, status)
	}
	if got := graphClient.FindAll(dag, &appsv1.Component{}); len(got) != 0 {
		t.Fatalf("failed OpsRequest scheduled %d marker writes", len(got))
	}
}

func TestManagedShardAddExplicitRetryWaitsForOldWorkloadAbsence(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status, live := bindManagedShardAddTestOpsRequest(t)
	job := attachManagedShardAddTestJob(t, fakeClient, live, opsv1alpha1.OpsFailedPhase)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: managedShardAddTestNamespace,
			Name:      "redis-shard-add-worker",
			UID:       types.UID("managed-worker-uid"),
			Labels: map[string]string{
				constant.ManagedJobSelectorLabelKey: opsutil.ManagedJobSelectorValue(string(live.UID), 0),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(job, batchv1.SchemeGroupVersion.WithKind(constant.JobKind)),
			},
		},
	}
	if err := fakeClient.Create(context.Background(), pod); err != nil {
		t.Fatal(err)
	}

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.JobUID != string(job.UID) {
		t.Fatalf("failed Job bind handled=%v err=%v status=%#v", handled, err, status)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil || status.Phase != appsv1.LifecycleActionFailed ||
		status.Reason != "OpsRequestUnsuccessful" {
		t.Fatalf("failed attempt handled=%v err=%v status=%#v", handled, err, status)
	}

	if err := fakeClient.Delete(context.Background(), live); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.Phase != appsv1.LifecycleActionFailed {
		t.Fatalf("old Job wait handled=%v err=%v status=%#v", handled, err, status)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("retry created %d OpsRequests while the old Job still existed", len(got))
	}

	if err := fakeClient.Delete(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.Phase != appsv1.LifecycleActionFailed {
		t.Fatalf("old Pod wait handled=%v err=%v status=%#v", handled, err, status)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("retry created %d OpsRequests while an old worker Pod still existed", len(got))
	}

	if err := fakeClient.Delete(context.Background(), pod); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.Phase != appsv1.LifecycleActionRunning ||
		status.Reason != "RetryPrepared" || status.OpsRequestUID != "" || status.JobUID != "" {
		t.Fatalf("retry preparation handled=%v err=%v status=%#v", handled, err, status)
	}

	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("retry create handled=%v err=%v", handled, err)
	}
	created := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{})
	if len(created) != 1 || created[0].GetName() != live.Name ||
		!graphClient.IsAction(dag, created[0], model.ActionCreatePtr()) {
		t.Fatalf("retry OpsRequest creates=%#v, want one same-name new attempt", created)
	}
}

func TestManagedShardAddMissingMarkerBeforeCleanupFailsClosed(t *testing.T) {
	ctx, graphClient, fakeClient, handler, status := managedShardAddReadyContext(t)
	member := &appsv1.Component{}
	key := client.ObjectKey{Namespace: managedShardAddTestNamespace, Name: managedShardAddTestMember}
	if err := fakeClient.Get(context.Background(), key, member); err != nil {
		t.Fatal(err)
	}
	delete(member.Annotations, shardingAddShardKey)
	if err := fakeClient.Update(context.Background(), member); err != nil {
		t.Fatal(err)
	}

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || err != nil || status.Phase != appsv1.LifecycleActionFailed ||
		status.Reason != "MemberIdentityMismatch" || status.CleanupStarted {
		t.Fatalf("pre-cleanup missing marker handled=%v err=%v status=%#v", handled, err, status)
	}
}

func TestManagedShardAddCleanupResumesAfterEachMemberPatch(t *testing.T) {
	const secondMember = "redis-redis-cluster-fghij"
	cluster := managedShardAddTestClusterObject()
	status, err := buildManagedShardAddPlan(cluster, managedShardAddTestSharding, 5,
		[]string{managedShardAddTestMember, secondMember})
	if err != nil {
		t.Fatal(err)
	}
	status.MembersDispatched = true
	status.CleanupStarted = true
	status.Phase = appsv1.LifecycleActionRunning
	status.Reason = "CleanupPending"
	status.OpsDefinitionName = managedShardAddTestOpsDef
	status.Members[0].UID = "component-uid-1"
	status.Members[1].UID = "component-uid-2"
	marker, err := encodeManagedShardAddMarker(markerForManagedShardAdd(cluster, managedShardAddTestSharding, status))
	if err != nil {
		t.Fatal(err)
	}
	memberOne := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: managedShardAddTestNamespace,
		Name:      managedShardAddTestMember,
		UID:       types.UID(status.Members[0].UID),
	}}
	memberTwo := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace:   managedShardAddTestNamespace,
		Name:        secondMember,
		UID:         types.UID(status.Members[1].UID),
		Annotations: map[string]string{shardingAddShardKey: marker},
	}}
	ctx, graphClient, fakeClient := managedShardAddTestContext(t, memberOne, memberTwo)
	ctx.Cluster.Status.Shardings[managedShardAddTestSharding] = appsv1.ClusterShardingStatus{ShardAdd: status}
	handler := &clusterShardingHandler{}

	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || status.MarkersCleaned {
		t.Fatalf("partial cleanup handled=%v err=%v status=%#v", handled, err, status)
	}
	patched := graphClient.FindAll(dag, &appsv1.Component{})
	if len(patched) != 1 || patched[0].GetName() != secondMember ||
		!graphClient.IsAction(dag, patched[0], model.ActionPatchPtr()) {
		t.Fatalf("partial cleanup patches=%#v, want only %s", patched, secondMember)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("cleanup resumed with %d OpsRequest writes", len(got))
	}
	if got := graphClient.FindAll(dag, &batchv1.Job{}); len(got) != 0 {
		t.Fatalf("cleanup resumed with %d Job writes", len(got))
	}

	liveSecond := &appsv1.Component{}
	if err := fakeClient.Get(context.Background(), client.ObjectKeyFromObject(memberTwo), liveSecond); err != nil {
		t.Fatal(err)
	}
	delete(liveSecond.Annotations, shardingAddShardKey)
	if err := fakeClient.Update(context.Background(), liveSecond); err != nil {
		t.Fatal(err)
	}
	dag = managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err = handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) || !status.MarkersCleaned ||
		status.Phase != appsv1.LifecycleActionSucceeded || status.CompletionTime == nil {
		t.Fatalf("cleanup close handled=%v err=%v status=%#v", handled, err, status)
	}
	if got := graphClient.FindAll(dag, &opsv1alpha1.OpsRequest{}); len(got) != 0 {
		t.Fatalf("cleanup close scheduled %d OpsRequest writes", len(got))
	}
	if got := graphClient.FindAll(dag, &batchv1.Job{}); len(got) != 0 {
		t.Fatalf("cleanup close scheduled %d Job writes", len(got))
	}
}

func TestManagedShardAddRecoversOneExactMarkerSet(t *testing.T) {
	cluster := managedShardAddTestClusterObject()
	status, err := buildManagedShardAddPlan(cluster, managedShardAddTestSharding, 4,
		[]string{managedShardAddTestMember})
	if err != nil {
		t.Fatal(err)
	}
	marker, err := encodeManagedShardAddMarker(markerForManagedShardAdd(cluster, managedShardAddTestSharding, status))
	if err != nil {
		t.Fatal(err)
	}
	member := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Name: managedShardAddTestMember,
		Annotations: map[string]string{
			shardingAddShardKey: marker,
		},
	}}
	ctx, graphClient, _ := managedShardAddTestContext(t)
	handler := &clusterShardingHandler{}
	dag := managedShardAddTestDAG(graphClient, ctx.Cluster)
	handled, err := handler.handleManagedShardAdd(ctx, dag, managedShardAddTestSharding,
		map[string]*appsv1.Component{member.Name: member}, map[string]*appsv1.Component{}, sets.New[string]())
	if !handled || !intctrlutil.IsDelayedRequeueError(err) {
		t.Fatalf("recovery handled=%v err=%v", handled, err)
	}
	recovered := ctx.Cluster.Status.Shardings[managedShardAddTestSharding].ShardAdd
	if recovered == nil || recovered.Reason != "PlanRecovered" || !recovered.MembersDispatched ||
		recovered.Token != status.Token || recovered.PlanHash != status.PlanHash {
		t.Fatalf("recovered status=%#v, source=%#v", recovered, status)
	}
}

func TestManagedShardAddOpsRequestParameterContract(t *testing.T) {
	ctx, _, _, _, status, expected := advanceManagedShardAddToOpsRequestPlan(t)
	parameters := expected.Spec.CustomOps.CustomOpsComponents[0].Parameters
	want := []opsv1alpha1.Parameter{
		{Name: "shardAddToken", Value: status.Token},
		{Name: "shardingName", Value: managedShardAddTestSharding},
		{Name: "newShardComponentNames", Value: managedShardAddTestMember},
		{Name: "targetShardCount", Value: "4"},
	}
	if !reflect.DeepEqual(parameters, want) {
		t.Fatalf("parameters=%#v, want %#v (cluster=%s)", parameters, want, ctx.Cluster.Name)
	}
}

func TestManagedShardAddRejectsCorruptPersistedTokenWithoutPanicking(t *testing.T) {
	cluster := managedShardAddTestClusterObject()
	status := &appsv1.ShardingActionStatus{
		Token:                   "short",
		MemberSnapshotHash:      strings.Repeat("a", 64),
		OpsDefinitionName:       managedShardAddTestOpsDef,
		OpsDefinitionUID:        "opsdef-uid",
		OpsDefinitionGeneration: 1,
		OpsDefinitionSpecHash:   strings.Repeat("b", 64),
		TargetShardCount:        4,
		Members:                 []appsv1.ShardingActionMemberStatus{{Name: managedShardAddTestMember}},
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("corrupt persisted token caused a panic: %v", recovered)
		}
	}()
	if _, _, err := buildManagedShardAddOpsRequest(cluster, managedShardAddTestSharding, status); err == nil {
		t.Fatal("corrupt persisted token was accepted")
	}
}

func TestManagedShardAddDoesNotInstallLegacyInlineAction(t *testing.T) {
	ctx, _, _ := managedShardAddTestContext(t)
	comp := &appsv1.Component{}
	(&clusterShardingHandler{}).buildShardingActions(ctx, &ctx.Cluster.Spec.Shardings[0], comp)
	for _, action := range comp.Spec.CustomActions {
		if action.Name == shardingAddShardAction {
			t.Fatal("managed shardAdd installed the legacy inline Component action")
		}
	}
	if got := comp.Annotations[constant.KubeBlocksGenerationKey]; got != "" {
		t.Fatalf("unexpected Component mutation while building actions: %q", got)
	}
}

func TestManagedShardAddProjectsClusterShardingStatusUntilCleanupCompletes(t *testing.T) {
	component := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Generation: 1,
			Annotations: map[string]string{
				constant.KubeBlocksGenerationKey: "2",
			},
		},
		Status: appsv1.ComponentStatus{
			Phase:              appsv1.RunningComponentPhase,
			ObservedGeneration: 1,
		},
	}
	tests := []struct {
		name         string
		action       *appsv1.ShardingActionStatus
		wantPhase    appsv1.ComponentPhase
		wantUpToDate bool
		wantReason   string
	}{
		{
			name: "running",
			action: &appsv1.ShardingActionStatus{LifecycleActionStatus: appsv1.LifecycleActionStatus{
				Phase: appsv1.LifecycleActionRunning, Reason: "ManagedJobRunning", Message: "waiting",
			}},
			wantPhase: appsv1.UpdatingComponentPhase, wantReason: "ManagedJobRunning",
		},
		{
			name: "cleanup pending",
			action: &appsv1.ShardingActionStatus{
				LifecycleActionStatus: appsv1.LifecycleActionStatus{
					Phase: appsv1.LifecycleActionRunning, Reason: "CleanupPending", Message: "persisted",
				},
				CleanupStarted: true,
			},
			wantPhase: appsv1.UpdatingComponentPhase, wantReason: "CleanupPending",
		},
		{
			name: "failed",
			action: &appsv1.ShardingActionStatus{LifecycleActionStatus: appsv1.LifecycleActionStatus{
				Phase: appsv1.LifecycleActionFailed, Reason: "OpsRequestUnsuccessful", Message: "failed",
			}},
			wantPhase: appsv1.FailedComponentPhase, wantReason: "OpsRequestUnsuccessful",
		},
		{
			name: "cleaned",
			action: &appsv1.ShardingActionStatus{
				LifecycleActionStatus: appsv1.LifecycleActionStatus{Phase: appsv1.LifecycleActionSucceeded},
				CleanupStarted:        true,
				MarkersCleaned:        true,
			},
			wantPhase: appsv1.RunningComponentPhase, wantUpToDate: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster := managedShardAddTestClusterObject()
			cluster.Status.Shardings[managedShardAddTestSharding] = appsv1.ClusterShardingStatus{
				Phase:    test.wantPhase,
				ShardAdd: test.action,
			}
			ctx := &clusterTransformContext{Cluster: cluster}
			got := (&clusterComponentStatusTransformer{}).buildClusterShardingStatus(ctx,
				managedShardAddTestSharding, []*appsv1.Component{component})
			if got.Phase != test.wantPhase || got.UpToDate != test.wantUpToDate || got.ShardAdd != test.action {
				t.Fatalf("status=%#v, want phase=%s upToDate=%v action preserved", got, test.wantPhase, test.wantUpToDate)
			}
			if test.wantReason != "" && got.Message["reason"] != test.wantReason {
				t.Fatalf("message=%#v, want reason=%q", got.Message, test.wantReason)
			}
		})
	}
}
