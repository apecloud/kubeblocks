/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

KubeBlocks is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

KubeBlocks is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with KubeBlocks.  If not, see <http://www.gnu.org/licenses/>.
*/

package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

type shardingScaleInInitialPlanTestSource struct {
	material *appsv1.ShardingScaleInPlanMaterial
	err      error
	calls    int
	reader   client.Reader
	key      types.NamespacedName
	uid      types.UID
	sharding string
}

type shardingScaleInConcurrentAuthorityReader struct {
	client.Reader
	writer             client.Client
	status             *appsv1.ShardingScaleInStatus
	lock               *appsv1.TopologyMutationLockStatus
	once               sync.Once
	err                error
	observedRV         string
	competingRV        string
	competingCommitted bool
}

func (r *shardingScaleInConcurrentAuthorityReader) Get(
	ctx context.Context,
	key client.ObjectKey,
	obj client.Object,
	opts ...client.GetOption,
) error {
	if err := r.Reader.Get(ctx, key, obj, opts...); err != nil {
		return err
	}
	cluster, ok := obj.(*appsv1.Cluster)
	if !ok {
		return nil
	}
	r.once.Do(func() {
		r.observedRV = cluster.ResourceVersion
		competing := &appsv1.Cluster{}
		if err := r.writer.Get(ctx, key, competing); err != nil {
			r.err = err
			return
		}
		if competing.Status.Shardings == nil {
			competing.Status.Shardings = map[string]appsv1.ClusterShardingStatus{}
		}
		competing.Status.Shardings["redis"] = appsv1.ClusterShardingStatus{
			ScaleIn: r.status.DeepCopy(),
		}
		competing.Status.TopologyMutationLock = r.lock.DeepCopy()
		if err := r.writer.Update(ctx, competing); err != nil {
			r.err = err
			return
		}
		readback := &appsv1.Cluster{}
		if err := r.writer.Get(ctx, key, readback); err != nil {
			r.err = err
			return
		}
		r.competingRV = readback.ResourceVersion
		r.competingCommitted = readback.Status.TopologyMutationLock != nil &&
			readback.Status.TopologyMutationLock.OwnerPlanID == r.lock.OwnerPlanID
	})
	return r.err
}

func (s *shardingScaleInInitialPlanTestSource) BuildInitialShardingScaleInPlanMaterial(
	_ context.Context,
	reader client.Reader,
	key types.NamespacedName,
	uid types.UID,
	shardingName string,
) (*appsv1.ShardingScaleInPlanMaterial, error) {
	s.calls++
	s.reader = reader
	s.key = key
	s.uid = uid
	s.sharding = shardingName
	if s.err != nil {
		return nil, s.err
	}
	if s.material == nil {
		return nil, nil
	}
	return s.material.DeepCopy(), nil
}

func TestTypedShardingScaleInInitialPlanCallsite(t *testing.T) {
	const shardingName = "redis"

	newCluster := func() *appsv1.Cluster {
		return &appsv1.Cluster{
			TypeMeta: metav1.TypeMeta{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ClusterKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "default",
				Name:            "demo",
				UID:             types.UID("cluster-uid"),
				ResourceVersion: "17",
				Generation:      7,
			},
			Spec: appsv1.ClusterSpec{
				Shardings: []appsv1.ClusterSharding{{
					Name:        shardingName,
					ShardingDef: "valkey-cluster",
					Shards:      2,
				}},
			},
		}
	}
	newContext := func(source shardingScaleInInitialPlanMaterialSource) (*clusterTransformContext, *graph.DAG) {
		scheme := runtime.NewScheme()
		if err := appsv1.AddToScheme(scheme); err != nil {
			t.Fatalf("add apps scheme: %v", err)
		}
		cluster := newCluster()
		shardingDef := &appsv1.ShardingDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "valkey-cluster",
				UID:        types.UID("sharding-definition-uid"),
				Generation: 3,
			},
			Spec: appsv1.ShardingDefinitionSpec{
				LifecycleActions: &appsv1.ShardingLifecycleActions{
					ShardRemove: &appsv1.ShardingAction{
						ResultProtocol: appsv1.ShardingScaleInResultProtocolV2,
					},
				},
			},
		}
		apiReader := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster.DeepCopy(), shardingDef.DeepCopy()).
			Build()
		graphCli := model.NewGraphClient(apiReader)
		dag := graph.NewDAG()
		graphCli.Root(dag, cluster.DeepCopy(), cluster, model.ActionStatusPtr())
		return &clusterTransformContext{
			Context:     context.Background(),
			Client:      graphCli,
			APIReader:   apiReader,
			Cluster:     cluster,
			OrigCluster: cluster.DeepCopy(),
			shardings: []*appsv1.ClusterSharding{{
				Name:        shardingName,
				ShardingDef: "valkey-cluster",
			}},
			shardingDefs: map[string]*appsv1.ShardingDefinition{
				"valkey-cluster": shardingDef.DeepCopy(),
			},
			shardingScaleInInitialPlanSource: source,
		}, dag
	}
	newMaterial := func() *appsv1.ShardingScaleInPlanMaterial {
		material := newShardingScaleInPlanMaterialFixture()
		material.ShardingName = shardingName
		canonical, _, err := buildShardingScaleInPlanMaterial(material)
		if err != nil {
			t.Fatalf("canonicalize initial PlanMaterial fixture: %v", err)
		}
		return canonical
	}
	updateLiveCluster := func(
		t *testing.T,
		transCtx *clusterTransformContext,
		mutate func(*appsv1.Cluster),
	) {
		t.Helper()
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveCluster := &appsv1.Cluster{}
		if err := apiWriter.Get(
			transCtx.Context,
			client.ObjectKeyFromObject(transCtx.Cluster),
			liveCluster,
		); err != nil {
			t.Fatalf("get live Cluster: %v", err)
		}
		mutate(liveCluster)
		if err := apiWriter.Update(transCtx.Context, liveCluster); err != nil {
			t.Fatalf("update live Cluster: %v", err)
		}
	}
	assertNoInitialPlanEffects := func(
		t *testing.T,
		source *shardingScaleInInitialPlanTestSource,
		dag *graph.DAG,
	) {
		t.Helper()
		if source.calls != 0 {
			t.Fatalf("fresh source calls = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent: intent=%v err=%v", intent, findErr)
		}
		for _, vertex := range dag.Vertices() {
			objectVertex, ok := vertex.(*model.ObjectVertex)
			if !ok {
				continue
			}
			if _, ok := objectVertex.Obj.(*appsv1.Component); ok {
				t.Fatal("Component effect was registered")
			}
		}
	}
	assertConcurrentOrdinaryPathIsFenced := func(
		t *testing.T,
		legacy bool,
		toDelete sets.Set[string],
	) {
		t.Helper()
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		apiWriterWithWatch, ok := apiWriter.(client.WithWatch)
		if !ok {
			t.Fatal("test APIReader does not implement client.WithWatch")
		}
		if legacy {
			transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
			liveShardingDef := &appsv1.ShardingDefinition{}
			if err := apiWriter.Get(
				transCtx.Context,
				client.ObjectKey{Name: "valkey-cluster"},
				liveShardingDef,
			); err != nil {
				t.Fatalf("get live ShardingDefinition: %v", err)
			}
			liveShardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
			if err := apiWriter.Update(transCtx.Context, liveShardingDef); err != nil {
				t.Fatalf("update live ShardingDefinition: %v", err)
			}
		}

		status, lock, err := buildInitialShardingScaleInState(
			newMaterial(),
			make([]byte, shardingScaleInTopologyFenceNonceBytes),
			&metav1.Time{Time: metav1.Now().Time},
		)
		if err != nil {
			t.Fatalf("build competing scale-in authority: %v", err)
		}
		raceReader := &shardingScaleInConcurrentAuthorityReader{
			Reader: apiWriter,
			writer: apiWriter,
			status: status,
			lock:   lock,
		}
		transCtx.APIReader = raceReader

		handled, reconcileErr := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, toDelete)

		ordinaryActionCalls := 0
		if !handled {
			// These model the immediate postProvision and generic shard-action
			// calls at the production callsite before Component DAG effects.
			ordinaryActionCalls += 2
			graphCli, ok := transCtx.Client.(model.GraphClient)
			if !ok {
				t.Fatal("test Client does not implement model.GraphClient")
			}
			graphCli.Create(dag, &appsv1.Component{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: transCtx.Cluster.Namespace,
					Name:      "must-not-be-created-after-authority-race",
				},
			})
		}

		conflict := apierrors.NewConflict(
			schema.GroupResource{Group: appsv1.GroupVersion.Group, Resource: "clusters"},
			transCtx.Cluster.Name,
			fmt.Errorf("resourceVersion changed after competing authority commit"),
		)
		executionClient := interceptor.NewClient(apiWriterWithWatch, interceptor.Funcs{
			SubResourcePatch: func(
				ctx context.Context,
				cli client.Client,
				subResourceName string,
				obj client.Object,
				patch client.Patch,
				opts ...client.SubResourcePatchOption,
			) error {
				if subResourceName == "status" && patch.Type() == types.JSONPatchType {
					return conflict
				}
				return cli.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
			},
		})
		builder := &clusterPlanBuilder{cli: executionClient, transCtx: transCtx}
		plan := &clusterPlan{
			dag:      dag,
			walkFunc: builder.defaultWalkFuncWithLogging,
			cli:      executionClient,
			transCtx: transCtx,
		}
		executeErr := plan.Execute()

		if !errors.Is(reconcileErr, graph.ErrPrematureStop) {
			t.Fatalf("ordinary path reconcile error = %v, want premature stop before stale CAS conflict",
				reconcileErr)
		}
		if !raceReader.competingCommitted {
			t.Fatal("competing RV11 scale-in authority was not committed")
		}
		if raceReader.observedRV == "" || raceReader.competingRV == "" ||
			raceReader.observedRV == raceReader.competingRV {
			t.Fatalf("authority interleaving RVs = observed:%q competing:%q, want distinct non-empty values",
				raceReader.observedRV, raceReader.competingRV)
		}
		if !handled {
			t.Error("ordinary path was not stopped by a stale authority-acquisition CAS")
		}
		if !apierrors.IsConflict(executeErr) {
			t.Errorf("execute error = %v, want competing-authority conflict", executeErr)
		}
		if ordinaryActionCalls != 0 {
			t.Errorf("ordinary action calls = %d, want postProvision/generic 0", ordinaryActionCalls)
		}
		created := &appsv1.Component{}
		getErr := apiWriter.Get(
			transCtx.Context,
			client.ObjectKey{
				Namespace: transCtx.Cluster.Namespace,
				Name:      "must-not-be-created-after-authority-race",
			},
			created,
		)
		if !apierrors.IsNotFound(getErr) {
			t.Errorf("Component read after conflict = %v, want NotFound", getErr)
		}
	}

	t.Run("registers one exclusive CAS intent from the fresh source and returns before effects", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if err != nil {
			t.Fatalf("reconcile initial plan: %v", err)
		}
		if !handled {
			t.Fatal("typed scale-in initial plan was not handled")
		}
		if source.calls != 1 || source.reader != transCtx.APIReader ||
			source.key != (types.NamespacedName{Namespace: "default", Name: "demo"}) ||
			source.uid != types.UID("cluster-uid") || source.sharding != shardingName {
			t.Fatalf("fresh source call = calls:%d reader:%t key:%v uid:%q sharding:%q",
				source.calls, source.reader == transCtx.APIReader, source.key, source.uid, source.sharding)
		}

		intent, err := findExclusiveClusterStatusCASVertex(dag)
		if err != nil {
			t.Fatalf("find exclusive status CAS: %v", err)
		}
		if intent == nil {
			t.Fatal("exclusive status CAS intent is missing")
		}
		if err := validateClusterStatusCASPatch(intent.cluster, intent.patch); err != nil {
			t.Fatalf("validate exclusive status CAS: %v", err)
		}
		if intent.cluster.ResourceVersion != "17" {
			t.Fatalf("CAS Cluster resourceVersion = %q, want fresh 17", intent.cluster.ResourceVersion)
		}
		for _, vertex := range dag.Vertices() {
			objectVertex, ok := vertex.(*model.ObjectVertex)
			if !ok {
				continue
			}
			if _, ok := objectVertex.Obj.(*appsv1.Component); ok {
				t.Fatal("Component effect was registered with initial status CAS")
			}
		}
	})

	t.Run("fails closed before an intent when the production source is unavailable", func(t *testing.T) {
		transCtx, dag := newContext(nil)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errShardingScaleInInitialPlanSourceUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/source unavailable", handled, err)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent after source failure: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("fails closed without an APIReader before deciding the legacy path", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		transCtx.APIReader = nil
		transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errShardingScaleInInitialPlanSourceUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/source unavailable", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls without APIReader = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent without APIReader: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("fails closed without an APIReader before accepting an empty diff", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		transCtx.APIReader = nil

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errShardingScaleInInitialPlanSourceUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/source unavailable", handled, err)
		}
		assertNoInitialPlanEffects(t, source, dag)
	})

	t.Run("fails closed when the source returns no material", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("handled/error = %t/%v, want true/invalid source", handled, err)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent after nil material: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("preserves source errors without registering an intent", func(t *testing.T) {
		sourceErr := errors.New("source read failed")
		source := &shardingScaleInInitialPlanTestSource{err: sourceErr}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, sourceErr) {
			t.Fatalf("handled/error = %t/%v, want true/source error", handled, err)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent after source error: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("rejects non-canonical source material", func(t *testing.T) {
		material := newMaterial()
		material.Staying[0], material.Staying[1] = material.Staying[1], material.Staying[0]
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("handled/error = %t/%v, want true/invalid source", handled, err)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent after non-canonical material: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("fails closed when returned material does not bind the fresh Cluster", func(t *testing.T) {
		material := newMaterial()
		material.Source.ClusterGeneration++
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("handled/error = %t/%v, want true/invalid fresh source", handled, err)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent after identity drift: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("never falls back to the legacy action for an existing typed plan", func(t *testing.T) {
		material := newMaterial()
		status, lock, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build existing state: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
			shardingName: {ScaleIn: status},
		}
		transCtx.Cluster.Status.TopologyMutationLock = lock

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for existing plan = %d, want 0", source.calls)
		}
	})

	t.Run("never lets an empty delete diff bypass an existing typed plan", func(t *testing.T) {
		material := newMaterial()
		status, lock, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build existing state: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		transCtx.Cluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
			shardingName: {ScaleIn: status},
		}
		transCtx.Cluster.Status.TopologyMutationLock = lock

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for existing plan with empty diff = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent for existing plan with empty diff: intent=%v err=%v", intent, findErr)
		}
		for _, vertex := range dag.Vertices() {
			objectVertex, ok := vertex.(*model.ObjectVertex)
			if !ok {
				continue
			}
			if _, ok := objectVertex.Obj.(*appsv1.Component); ok {
				t.Fatal("Component effect was registered for an existing plan with empty diff")
			}
		}
	})

	t.Run("fails closed on a topology lock without a usable plan", func(t *testing.T) {
		material := newMaterial()
		_, lock, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build existing lock: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		transCtx.Cluster.Status.TopologyMutationLock = lock

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls with existing topology lock = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent with existing topology lock: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("does not let an empty diff bypass a fresh lock hidden from the cache", func(t *testing.T) {
		material := newMaterial()
		_, lock, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build fresh lock: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		updateLiveCluster(t, transCtx, func(liveCluster *appsv1.Cluster) {
			liveCluster.Status.TopologyMutationLock = lock
		})

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		assertNoInitialPlanEffects(t, source, dag)
	})

	t.Run("does not let a legacy diff bypass a fresh lock hidden from the cache", func(t *testing.T) {
		material := newMaterial()
		_, lock, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build fresh lock: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveShardingDef := &appsv1.ShardingDefinition{}
		if err := apiWriter.Get(transCtx.Context, client.ObjectKey{Name: "valkey-cluster"}, liveShardingDef); err != nil {
			t.Fatalf("get live ShardingDefinition: %v", err)
		}
		liveShardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		if err := apiWriter.Update(transCtx.Context, liveShardingDef); err != nil {
			t.Fatalf("update live ShardingDefinition: %v", err)
		}
		updateLiveCluster(t, transCtx, func(liveCluster *appsv1.Cluster) {
			liveCluster.Status.TopologyMutationLock = lock
		})

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		assertNoInitialPlanEffects(t, source, dag)
	})

	t.Run("does not let an empty diff bypass a fresh plan hidden from the cache", func(t *testing.T) {
		material := newMaterial()
		status, _, err := buildInitialShardingScaleInState(
			material, make([]byte, shardingScaleInTopologyFenceNonceBytes), &metav1.Time{Time: metav1.Now().Time})
		if err != nil {
			t.Fatalf("build fresh plan: %v", err)
		}
		source := &shardingScaleInInitialPlanTestSource{material: material}
		transCtx, dag := newContext(source)
		updateLiveCluster(t, transCtx, func(liveCluster *appsv1.Cluster) {
			liveCluster.Status.Shardings = map[string]appsv1.ClusterShardingStatus{
				shardingName: {ScaleIn: status},
			}
		})

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
			t.Fatalf("handled/error = %t/%v, want true/progression unavailable", handled, err)
		}
		assertNoInitialPlanEffects(t, source, dag)
	})

	t.Run("fences empty-path effects when another reconcile commits authority after the fresh read", func(t *testing.T) {
		assertConcurrentOrdinaryPathIsFenced(t, false, nil)
	})

	t.Run("fences legal-legacy effects when another reconcile commits authority after the fresh read", func(t *testing.T) {
		assertConcurrentOrdinaryPathIsFenced(t, true, sets.New("demo-redis-2"))
	})

	t.Run("allows empty-path effects only under the exact persisted ordinary lock", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		lock, err := buildClusterTopologyReconcileLock(transCtx.Cluster)
		if err != nil {
			t.Fatalf("build ordinary lock: %v", err)
		}
		liveCluster := &appsv1.Cluster{}
		key := client.ObjectKeyFromObject(transCtx.Cluster)
		if err := apiWriter.Get(transCtx.Context, key, liveCluster); err != nil {
			t.Fatalf("get live Cluster: %v", err)
		}
		liveCluster.Status.TopologyMutationLock = lock
		if err := apiWriter.Update(transCtx.Context, liveCluster); err != nil {
			t.Fatalf("persist ordinary lock: %v", err)
		}
		if err := apiWriter.Get(transCtx.Context, key, liveCluster); err != nil {
			t.Fatalf("read persisted ordinary lock: %v", err)
		}
		transCtx.Cluster = liveCluster.DeepCopy()
		transCtx.OrigCluster = liveCluster.DeepCopy()

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if handled || err != nil {
			t.Fatalf("owned ordinary path handled/error = %t/%v, want false/nil", handled, err)
		}
		if transCtx.clusterTopologyReconcileLockRelease == nil ||
			transCtx.clusterTopologyReconcileLockRelease.FenceToken !=
				liveCluster.Status.TopologyMutationLock.FenceToken {
			t.Fatal("owned ordinary lock was not bound to the post-execution release")
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS while executing under the owned lock: intent=%v err=%v",
				intent, findErr)
		}
	})

	t.Run("releases the ordinary lock before installing a typed plan", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		lock, err := buildClusterTopologyReconcileLock(transCtx.Cluster)
		if err != nil {
			t.Fatalf("build ordinary lock: %v", err)
		}
		liveCluster := &appsv1.Cluster{}
		key := client.ObjectKeyFromObject(transCtx.Cluster)
		if err := apiWriter.Get(transCtx.Context, key, liveCluster); err != nil {
			t.Fatalf("get live Cluster: %v", err)
		}
		liveCluster.Status.TopologyMutationLock = lock
		if err := apiWriter.Update(transCtx.Context, liveCluster); err != nil {
			t.Fatalf("persist ordinary lock: %v", err)
		}
		if err := apiWriter.Get(transCtx.Context, key, liveCluster); err != nil {
			t.Fatalf("read persisted ordinary lock: %v", err)
		}
		transCtx.Cluster = liveCluster.DeepCopy()
		transCtx.OrigCluster = liveCluster.DeepCopy()

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || err != nil {
			t.Fatalf("typed path under ordinary lock handled/error = %t/%v, want true/nil",
				handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("typed source calls before ordinary lock release = %d, want 0", source.calls)
		}
		if transCtx.clusterTopologyReconcileLockRelease == nil {
			t.Fatal("ordinary lock release was not requested before typed plan installation")
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected typed CAS before ordinary lock release: intent=%v err=%v",
				intent, findErr)
		}
	})

	t.Run("fails closed on fresh Cluster identity drift before accepting an empty diff", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		updateLiveCluster(t, transCtx, func(liveCluster *appsv1.Cluster) {
			liveCluster.Generation++
		})

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("handled/error = %t/%v, want true/invalid source", handled, err)
		}
		assertNoInitialPlanEffects(t, source, dag)
	})

	t.Run("does not trust a cached legacy protocol when the live definition is typed", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if err != nil {
			t.Fatalf("reconcile live typed definition: %v", err)
		}
		if !handled {
			t.Fatal("live typed definition fell through to the cached legacy path")
		}
		if source.calls != 1 {
			t.Fatalf("fresh source calls = %d, want 1", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent == nil {
			t.Fatalf("typed CAS intent = %v, err=%v", intent, findErr)
		}
	})

	t.Run("does not trust a cached typed protocol when the live definition is legacy", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveShardingDef := &appsv1.ShardingDefinition{}
		if err := apiWriter.Get(transCtx.Context, client.ObjectKey{Name: "valkey-cluster"}, liveShardingDef); err != nil {
			t.Fatalf("get live ShardingDefinition: %v", err)
		}
		liveShardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		if err := apiWriter.Update(transCtx.Context, liveShardingDef); err != nil {
			t.Fatalf("update live ShardingDefinition: %v", err)
		}

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("live legacy handled/error = %t/%v, want true/invalid source", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for live legacy definition = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent for stale live legacy definition: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("installs ordinary authority before an inline legacy sharding path", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveCluster := &appsv1.Cluster{}
		if err := apiWriter.Get(
			transCtx.Context,
			client.ObjectKeyFromObject(transCtx.Cluster),
			liveCluster,
		); err != nil {
			t.Fatalf("get live Cluster: %v", err)
		}
		liveCluster.Spec.Shardings[0].ShardingDef = ""
		if err := apiWriter.Update(transCtx.Context, liveCluster); err != nil {
			t.Fatalf("update live Cluster: %v", err)
		}
		transCtx.Cluster.Generation = liveCluster.Generation
		transCtx.Cluster.Spec.Shardings[0].ShardingDef = ""
		transCtx.shardings[0].ShardingDef = ""

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, graph.ErrPrematureStop) {
			t.Fatalf("inline sharding handled/error = %t/%v, want true/premature-stop",
				handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for inline sharding = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent == nil {
			t.Fatalf("ordinary authority CAS intent = %v, err=%v", intent, findErr)
		}
	})

	t.Run("fails closed when topology hides the effective sharding definition selector", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveCluster := &appsv1.Cluster{}
		if err := apiWriter.Get(
			transCtx.Context,
			client.ObjectKeyFromObject(transCtx.Cluster),
			liveCluster,
		); err != nil {
			t.Fatalf("get live Cluster: %v", err)
		}
		liveCluster.Spec.Topology = "topology-with-typed-sharding"
		liveCluster.Spec.Shardings[0].ShardingDef = ""
		if err := apiWriter.Update(transCtx.Context, liveCluster); err != nil {
			t.Fatalf("update live Cluster: %v", err)
		}
		transCtx.Cluster.Generation = liveCluster.Generation
		transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, errInvalidShardingScaleInInitialPlanSource) {
			t.Fatalf("topology-hidden selector handled/error = %t/%v, want true/invalid source", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for topology-hidden selector = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent != nil {
			t.Fatalf("unexpected CAS intent for topology-hidden selector: intent=%v err=%v", intent, findErr)
		}
	})

	t.Run("installs one ordinary authority before legacy effects", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)
		transCtx.shardingDefs["valkey-cluster"].Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		apiWriter, ok := transCtx.APIReader.(client.Client)
		if !ok {
			t.Fatal("test APIReader does not implement client.Client")
		}
		liveShardingDef := &appsv1.ShardingDefinition{}
		if err := apiWriter.Get(transCtx.Context, client.ObjectKey{Name: "valkey-cluster"}, liveShardingDef); err != nil {
			t.Fatalf("get live ShardingDefinition: %v", err)
		}
		liveShardingDef.Spec.LifecycleActions.ShardRemove.ResultProtocol = ""
		if err := apiWriter.Update(transCtx.Context, liveShardingDef); err != nil {
			t.Fatalf("update live ShardingDefinition: %v", err)
		}

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, sets.New("demo-redis-2"))
		if !handled || !errors.Is(err, graph.ErrPrematureStop) {
			t.Fatalf("legacy handled/error = %t/%v, want true/premature-stop", handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for legacy path = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent == nil {
			t.Fatalf("ordinary authority CAS intent = %v, err=%v", intent, findErr)
		}
	})

	t.Run("installs one ordinary authority before no-delete effects", func(t *testing.T) {
		source := &shardingScaleInInitialPlanTestSource{material: newMaterial()}
		transCtx, dag := newContext(source)

		handled, err := (&clusterShardingHandler{}).reconcileTypedScaleInInitialPlan(
			transCtx, dag, shardingName, nil)
		if !handled || !errors.Is(err, graph.ErrPrematureStop) {
			t.Fatalf("no-delete handled/error = %t/%v, want true/premature-stop",
				handled, err)
		}
		if source.calls != 0 {
			t.Fatalf("fresh source calls for no-delete path = %d, want 0", source.calls)
		}
		intent, findErr := findExclusiveClusterStatusCASVertex(dag)
		if findErr != nil || intent == nil {
			t.Fatalf("ordinary authority CAS intent = %v, err=%v", intent, findErr)
		}
	})
}
