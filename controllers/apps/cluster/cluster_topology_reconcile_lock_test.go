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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func newClusterTopologyReconcileLockTestCluster() *appsv1.Cluster {
	return &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       appsv1.ClusterKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "cluster",
			UID:             types.UID("cluster-uid"),
			ResourceVersion: "7",
			Generation:      3,
		},
	}
}

func TestClusterTopologyReconcileLockValidation(t *testing.T) {
	cluster := newClusterTopologyReconcileLockTestCluster()
	lock, err := buildClusterTopologyReconcileLock(cluster)
	if err != nil {
		t.Fatalf("build lock: %v", err)
	}
	if err := validateClusterTopologyReconcileLock(cluster, lock); err != nil {
		t.Fatalf("validate lock: %v", err)
	}
	if lock.OwnerPlanID != clusterTopologyReconcilePlanID(cluster.UID) {
		t.Fatalf("owner plan ID = %q, want stable Cluster identity", lock.OwnerPlanID)
	}
	if len(lock.AffectedComponentUIDs) != 0 {
		t.Fatalf("affected Component UIDs = %v, want cluster-wide empty scope",
			lock.AffectedComponentUIDs)
	}

	tests := []struct {
		name   string
		mutate func(*appsv1.TopologyMutationLockStatus)
	}{
		{
			name: "empty fence",
			mutate: func(lock *appsv1.TopologyMutationLockStatus) {
				lock.FenceToken = ""
			},
		},
		{
			name: "other Cluster",
			mutate: func(lock *appsv1.TopologyMutationLockStatus) {
				lock.ClusterUID = "other"
			},
		},
		{
			name: "other owner plan",
			mutate: func(lock *appsv1.TopologyMutationLockStatus) {
				lock.OwnerPlanID = "other"
			},
		},
		{
			name: "wrong state",
			mutate: func(lock *appsv1.TopologyMutationLockStatus) {
				lock.State = appsv1.TopologyMutationLockStateReleaseReady
			},
		},
		{
			name: "narrowed Components",
			mutate: func(lock *appsv1.TopologyMutationLockStatus) {
				lock.AffectedComponentUIDs = []types.UID{"component"}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tampered := lock.DeepCopy()
			test.mutate(tampered)
			if err := validateClusterTopologyReconcileLock(cluster, tampered); err == nil {
				t.Fatal("tampered lock passed validation")
			}
		})
	}
}

func TestClusterTopologyAuthorityTransformerBlocksForeignLockBeforeEffects(t *testing.T) {
	cluster := newClusterTopologyReconcileLockTestCluster()
	cluster.Status.TopologyMutationLock = &appsv1.TopologyMutationLockStatus{
		OwnerKind:   appsv1.TopologyMutationLockOwnerShardingScaleIn,
		OwnerPlanID: "typed-plan",
		State:       appsv1.TopologyMutationLockStateInstallingAuthority,
	}
	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Cluster:     cluster,
		OrigCluster: cluster.DeepCopy(),
	}
	// The immutable input remains authoritative even if an earlier transformer
	// mutates the working status copy.
	transCtx.Cluster.Status.TopologyMutationLock = nil
	err := (&clusterTopologyReconcileAuthorityTransformer{}).Transform(transCtx, graph.NewDAG())
	if !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
		t.Fatalf("transform error = %v, want topology authority barrier", err)
	}
}

type clusterTopologyReconcileAcquireTestTransformer struct {
	freshCluster *appsv1.Cluster
}

func (t *clusterTopologyReconcileAcquireTestTransformer) Transform(
	ctx graph.TransformContext,
	dag *graph.DAG,
) error {
	return addClusterTopologyReconcileLockIntent(
		ctx.(*clusterTransformContext), dag, t.freshCluster)
}

type clusterTopologyReconcileEffectTestTransformer struct {
	calls int
}

func (t *clusterTopologyReconcileEffectTestTransformer) Transform(
	ctx graph.TransformContext,
	dag *graph.DAG,
) error {
	t.calls++
	ctx.(*clusterTransformContext).Client.(model.GraphClient).Create(dag, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "must-not-run-after-lock-acquisition",
		},
	})
	return nil
}

type clusterTopologyReconcilePreAuthorityEffectTestTransformer struct {
	name string
	err  error
}

func (t *clusterTopologyReconcilePreAuthorityEffectTestTransformer) Transform(
	ctx graph.TransformContext,
	dag *graph.DAG,
) error {
	ctx.(*clusterTransformContext).Client.(model.GraphClient).Create(dag, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      t.name,
		},
	})
	return t.err
}

func TestClusterPlanBlocksPreComponentEffectsWithoutWholePlanAuthority(t *testing.T) {
	ordinaryErr := errors.New("pre-component transformer failed after registering an effect")
	tests := []struct {
		name       string
		lock       func(*testing.T, *appsv1.Cluster) *appsv1.TopologyMutationLockStatus
		effectErr  error
		wantErr    error
		wantHeld   bool
		wantNoLock bool
	}{
		{
			name: "foreign lock",
			lock: func(
				_ *testing.T,
				cluster *appsv1.Cluster,
			) *appsv1.TopologyMutationLockStatus {
				return &appsv1.TopologyMutationLockStatus{
					Version:               appsv1.TopologyMutationLockVersionV1,
					FenceToken:            strings.Repeat("a", 64),
					ClusterUID:            cluster.UID,
					OwnerKind:             appsv1.TopologyMutationLockOwnerShardingScaleIn,
					OwnerPlanID:           "foreign-plan",
					State:                 appsv1.TopologyMutationLockStateInstallingAuthority,
					AcquiredAt:            &metav1.Time{Time: time.Unix(1, 0)},
					AffectedComponentUIDs: []types.UID{"component"},
				}
			},
			wantErr: errShardingScaleInPlanProgressionUnavailable,
		},
		{
			name: "malformed ordinary lock",
			lock: func(
				t *testing.T,
				cluster *appsv1.Cluster,
			) *appsv1.TopologyMutationLockStatus {
				t.Helper()
				lock, err := buildClusterTopologyReconcileLock(cluster)
				if err != nil {
					t.Fatal(err)
				}
				lock.FenceToken = ""
				return lock
			},
			wantErr: errInvalidClusterTopologyReconcileLock,
		},
		{
			name: "exact ordinary lock and ordinary build error",
			lock: func(
				t *testing.T,
				cluster *appsv1.Cluster,
			) *appsv1.TopologyMutationLockStatus {
				t.Helper()
				lock, err := buildClusterTopologyReconcileLock(cluster)
				if err != nil {
					t.Fatal(err)
				}
				return lock
			},
			effectErr: ordinaryErr,
			wantErr:   ordinaryErr,
			wantHeld:  true,
		},
		{
			name:       "no lock and ordinary build error",
			effectErr:  ordinaryErr,
			wantErr:    ordinaryErr,
			wantNoLock: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			if err := corev1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			if err := appsv1.AddToScheme(scheme); err != nil {
				t.Fatal(err)
			}
			cluster := newClusterTopologyReconcileLockTestCluster()
			if test.lock != nil {
				cluster.Status.TopologyMutationLock = test.lock(t, cluster)
			}
			base := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(cluster).
				WithObjects(cluster).
				Build()
			persisted := &appsv1.Cluster{}
			if err := base.Get(
				context.Background(),
				client.ObjectKeyFromObject(cluster),
				persisted,
			); err != nil {
				t.Fatal(err)
			}

			graphCli := model.NewGraphClient(base)
			dag := graph.NewDAG()
			graphCli.Root(dag, persisted.DeepCopy(), persisted.DeepCopy(), model.ActionStatusPtr())
			transCtx := &clusterTransformContext{
				Context:     context.Background(),
				Client:      graphCli,
				APIReader:   base,
				Logger:      logr.Discard(),
				Cluster:     persisted.DeepCopy(),
				OrigCluster: persisted.DeepCopy(),
			}
			effectName := "pre-authority-" + strings.ReplaceAll(test.name, " ", "-")
			chain := graph.TransformerChain{
				&clusterTopologyReconcileAuthorityTransformer{},
				&clusterTopologyReconcilePreAuthorityEffectTestTransformer{
					name: effectName,
					err:  test.effectErr,
				},
				&clusterComponentTransformer{},
			}
			buildErr := chain.ApplyTo(transCtx, dag)
			if !errors.Is(buildErr, test.wantErr) {
				t.Fatalf("build error = %v, want %v", buildErr, test.wantErr)
			}
			builder := &clusterPlanBuilder{cli: base, transCtx: transCtx}
			plan := &clusterPlan{
				dag:      dag,
				walkFunc: builder.defaultWalkFuncWithLogging,
				cli:      base,
				transCtx: transCtx,
				buildErr: buildErr,
			}
			if err := plan.Execute(); !errors.Is(err, test.wantErr) {
				t.Fatalf("execute error = %v, want %v", err, test.wantErr)
			}

			effect := &corev1.ConfigMap{}
			getErr := base.Get(context.Background(), client.ObjectKey{
				Namespace: cluster.Namespace,
				Name:      effectName,
			}, effect)
			if !apierrors.IsNotFound(getErr) {
				t.Fatalf("pre-authority effect read = %v, want NotFound", getErr)
			}
			readback := &appsv1.Cluster{}
			if err := base.Get(
				context.Background(),
				client.ObjectKeyFromObject(cluster),
				readback,
			); err != nil {
				t.Fatal(err)
			}
			if test.wantHeld &&
				(readback.Status.TopologyMutationLock == nil ||
					readback.Status.TopologyMutationLock.State != appsv1.TopologyMutationLockStateHeld) {
				t.Fatalf("ordinary lock after blocked build = %#v, want retained Held",
					readback.Status.TopologyMutationLock)
			}
			if test.wantNoLock && readback.Status.TopologyMutationLock != nil {
				t.Fatalf("topology lock after blocked no-lock build = %#v, want nil",
					readback.Status.TopologyMutationLock)
			}
		})
	}
}

func TestClusterTopologyReconcileAcquisitionStopsChainAndPersistsLock(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := newClusterTopologyReconcileLockTestCluster()
	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(cluster).
		WithObjects(cluster).
		Build()
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(cluster),
		cluster,
	); err != nil {
		t.Fatalf("read persisted Cluster: %v", err)
	}

	graphCli := model.NewGraphClient(base)
	dag := graph.NewDAG()
	graphCli.Root(dag, cluster.DeepCopy(), cluster.DeepCopy(), model.ActionStatusPtr())
	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Client:      graphCli,
		APIReader:   base,
		Logger:      logr.Discard(),
		Cluster:     cluster.DeepCopy(),
		OrigCluster: cluster.DeepCopy(),
	}
	acquire := &clusterTopologyReconcileAcquireTestTransformer{
		freshCluster: cluster.DeepCopy(),
	}
	effect := &clusterTopologyReconcileEffectTestTransformer{}
	if err := (graph.TransformerChain{acquire, effect}).ApplyTo(transCtx, dag); err != nil {
		t.Fatalf("apply acquisition chain: %v", err)
	}
	if effect.calls != 0 {
		t.Fatalf("transformers after acquisition = %d, want 0", effect.calls)
	}
	if !transCtx.clusterTopologyReconcileLockAcquisitionPending {
		t.Fatal("ordinary lock acquisition was not marked pending")
	}
	intent, err := findExclusiveClusterStatusCASVertex(dag)
	if err != nil || intent == nil {
		t.Fatalf("ordinary acquisition CAS intent = %v, err=%v", intent, err)
	}

	builder := &clusterPlanBuilder{cli: base, transCtx: transCtx}
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: builder.defaultWalkFuncWithLogging,
		cli:      base,
		transCtx: transCtx,
	}
	if err := plan.Execute(); err != nil {
		t.Fatalf("execute ordinary lock acquisition: %v", err)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(cluster),
		readback,
	); err != nil {
		t.Fatalf("read acquired lock: %v", err)
	}
	if err := validateClusterTopologyReconcileLock(
		readback, readback.Status.TopologyMutationLock); err != nil {
		t.Fatalf("persisted ordinary lock: %v", err)
	}
	blockedEffect := &corev1.ConfigMap{}
	getErr := base.Get(context.Background(), client.ObjectKey{
		Namespace: "default",
		Name:      "must-not-run-after-lock-acquisition",
	}, blockedEffect)
	if !apierrors.IsNotFound(getErr) {
		t.Fatalf("effect after acquisition read = %v, want NotFound", getErr)
	}
}

type clusterTopologyReconcilePlanCounters struct {
	creates           int
	statusJSONPatches int
	statusPatchData   []byte
	createErr         error
}

type clusterTopologyReconcilePlanSourceError struct {
	err error
}

func (s *clusterTopologyReconcilePlanSourceError) BuildInitialShardingScaleInPlanMaterial(
	context.Context,
	client.Reader,
	types.NamespacedName,
	types.UID,
	string,
) (*appsv1.ShardingScaleInPlanMaterial, error) {
	return nil, s.err
}

func newClusterTopologyReconcileReleasePlan(
	t *testing.T,
	buildErr error,
	createErr error,
) (*clusterPlan, client.Client, *clusterTopologyReconcilePlanCounters) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := newClusterTopologyReconcileLockTestCluster()
	lock, err := buildClusterTopologyReconcileLock(cluster)
	if err != nil {
		t.Fatalf("build lock: %v", err)
	}
	cluster.Status.TopologyMutationLock = lock
	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(cluster).
		WithObjects(cluster).
		Build()
	persistedCluster := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(cluster),
		persistedCluster,
	); err != nil {
		t.Fatalf("read persisted Cluster: %v", err)
	}
	cluster = persistedCluster
	lock = cluster.Status.TopologyMutationLock.DeepCopy()
	counters := &clusterTopologyReconcilePlanCounters{createErr: createErr}
	cli := interceptor.NewClient(base, interceptor.Funcs{
		Create: func(
			ctx context.Context,
			cli client.WithWatch,
			obj client.Object,
			opts ...client.CreateOption,
		) error {
			counters.creates++
			if counters.createErr != nil {
				return counters.createErr
			}
			return cli.Create(ctx, obj, opts...)
		},
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			if subResourceName == "status" && patch.Type() == types.JSONPatchType {
				counters.statusJSONPatches++
				data, err := patch.Data(obj)
				if err != nil {
					return err
				}
				counters.statusPatchData = append([]byte(nil), data...)
			}
			return cli.SubResource(subResourceName).Patch(ctx, obj, patch, opts...)
		},
	})

	dag := graph.NewDAG()
	root := model.NewObjectVertex(cluster.DeepCopy(), cluster.DeepCopy(), model.ActionStatusPtr())
	dag.AddVertex(root)
	dag.AddConnect(root, model.NewObjectVertex(nil, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Namespace,
			Name:      "ordinary-effect",
		},
	}, model.ActionCreatePtr()))
	transCtx := &clusterTransformContext{
		Context:                               context.Background(),
		Client:                                model.NewGraphClient(base),
		APIReader:                             base,
		Logger:                                logr.Discard(),
		Cluster:                               cluster.DeepCopy(),
		OrigCluster:                           cluster.DeepCopy(),
		clusterTopologyReconcileLockAuthority: lock.DeepCopy(),
		clusterTopologyReconcileLockRelease:   lock.DeepCopy(),
	}
	builder := &clusterPlanBuilder{cli: cli, transCtx: transCtx}
	return &clusterPlan{
		dag:      dag,
		walkFunc: builder.defaultWalkFuncWithLogging,
		cli:      cli,
		transCtx: transCtx,
		buildErr: buildErr,
	}, base, counters
}

func TestClusterPlanReleasesOrdinaryTopologyLockAfterSuccessfulEffects(t *testing.T) {
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, nil)

	if err := plan.Execute(); err != nil {
		t.Fatalf("execute plan: %v", err)
	}
	if counters.creates != 1 || counters.statusJSONPatches != 3 {
		t.Fatalf("writes = creates:%d authorityAndReleaseCAS:%d, want 1/3",
			counters.creates, counters.statusJSONPatches)
	}
	operations := []clusterStatusCASOperation{}
	if err := json.Unmarshal(counters.statusPatchData, &operations); err != nil {
		t.Fatalf("decode release patch: %v", err)
	}
	if len(operations) != 4 ||
		operations[2].Operation != "test" ||
		operations[2].Path != "/status/topologyMutationLock" ||
		operations[3].Operation != "remove" ||
		operations[3].Path != "/status/topologyMutationLock" {
		t.Fatalf("release patch = %s, want UID/RV/exact-lock tests then remove",
			counters.statusPatchData)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(plan.transCtx.Cluster), readback); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock != nil {
		t.Fatalf("topology lock after successful release = %#v, want nil",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterPlanOrdinaryBuildErrorBlocksPartialEffects(t *testing.T) {
	buildErr := errors.New("build failed after partial DAG")
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)

	if err := plan.Execute(); !errors.Is(err, buildErr) {
		t.Fatalf("execute error = %v, want build error", err)
	}
	if counters.creates != 0 || counters.statusJSONPatches != 0 {
		t.Fatalf("writes = creates:%d authorityCAS:%d, want 0/0",
			counters.creates, counters.statusJSONPatches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(plan.transCtx.Cluster), readback); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock == nil ||
		readback.Status.TopologyMutationLock.State != appsv1.TopologyMutationLockStateHeld {
		t.Fatalf("topology lock after Build error = %#v, want retained Held receipt",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterPlanOrdinaryBuildErrorPersistsOnlyFailureConditions(t *testing.T) {
	buildErr := errors.New("build failed after mutating the transform copy")
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)
	plan.transCtx.Cluster.Labels = map[string]string{"must-not-persist": "true"}
	plan.transCtx.Cluster.Spec.TerminationPolicy = appsv1.WipeOut
	failureCondition := metav1.Condition{
		Type:    appsv1.ConditionTypeProvisioningStarted,
		Status:  metav1.ConditionFalse,
		Reason:  ReasonPreCheckFailed,
		Message: buildErr.Error(),
	}
	meta.SetStatusCondition(&plan.transCtx.Cluster.Status.Conditions, failureCondition)
	meta.SetStatusCondition(&plan.transCtx.Cluster.Status.Conditions, metav1.Condition{
		Type:    appsv1.ConditionTypeReady,
		Status:  metav1.ConditionTrue,
		Reason:  ReasonClusterReady,
		Message: "must not persist through the failure-only compatibility path",
	})

	if err := plan.Execute(); !errors.Is(err, buildErr) {
		t.Fatalf("execute error = %v, want build error", err)
	}
	if counters.creates != 0 || counters.statusJSONPatches != 0 {
		t.Fatalf("business writes = creates:%d authorityCAS:%d, want 0/0",
			counters.creates, counters.statusJSONPatches)
	}

	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(plan.transCtx.Cluster), readback); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Labels["must-not-persist"] != "" {
		t.Fatalf("transform-only label persisted: %#v", readback.Labels)
	}
	if readback.Spec.TerminationPolicy == appsv1.WipeOut {
		t.Fatalf("transform-only spec persisted: %#v", readback.Spec)
	}
	if got := meta.FindStatusCondition(
		readback.Status.Conditions,
		appsv1.ConditionTypeProvisioningStarted,
	); got == nil || got.Status != metav1.ConditionFalse || got.Reason != ReasonPreCheckFailed {
		t.Fatalf("failure condition = %#v, want persisted pre-check failure", got)
	}
	if got := meta.FindStatusCondition(
		readback.Status.Conditions,
		appsv1.ConditionTypeReady,
	); got != nil {
		t.Fatalf("non-failure condition persisted: %#v", got)
	}
	if readback.Status.TopologyMutationLock == nil ||
		readback.Status.TopologyMutationLock.State != appsv1.TopologyMutationLockStateHeld {
		t.Fatalf("topology lock after Build error = %#v, want retained Held receipt",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterPlanOrdinaryBuildErrorRejectsConcurrentStatusCommit(t *testing.T) {
	buildErr := errors.New("ordinary build failure")
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)
	meta.SetStatusCondition(&plan.transCtx.Cluster.Status.Conditions,
		newFailedApplyResourcesCondition(buildErr))

	fresh := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(plan.transCtx.Cluster)
	if err := base.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	concurrentCondition := metav1.Condition{
		Type:    "ConcurrentWriter",
		Status:  metav1.ConditionTrue,
		Reason:  "CommittedAfterBuild",
		Message: "must survive the stale failure-condition writer",
	}
	meta.SetStatusCondition(&fresh.Status.Conditions, concurrentCondition)
	if err := base.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("commit concurrent status: %v", err)
	}

	err := plan.Execute()
	if !apierrors.IsConflict(err) {
		t.Fatalf("execute error = %v, want Conflict", err)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), key, readback); err != nil {
		t.Fatalf("read Cluster after conflict: %v", err)
	}
	if got := meta.FindStatusCondition(
		readback.Status.Conditions,
		concurrentCondition.Type,
	); got == nil || got.Reason != concurrentCondition.Reason {
		t.Fatalf("concurrent condition = %#v, want preserved", got)
	}
	if got := meta.FindStatusCondition(
		readback.Status.Conditions,
		appsv1.ConditionTypeApplyResources,
	); got != nil {
		t.Fatalf("stale failure condition persisted after conflict: %#v", got)
	}
}

func TestClusterPlanOrdinaryBuildErrorReturnsStatusPatchError(t *testing.T) {
	buildErr := errors.New("ordinary build failure")
	tests := []struct {
		name       string
		patchErr   error
		matchesErr func(error) bool
	}{
		{
			name: "not found",
			patchErr: apierrors.NewNotFound(
				schema.GroupResource{
					Group:    appsv1.GroupVersion.Group,
					Resource: "clusters",
				},
				"cluster",
			),
			matchesErr: apierrors.IsNotFound,
		},
		{
			name:       "server error",
			patchErr:   apierrors.NewInternalError(errors.New("status unavailable")),
			matchesErr: apierrors.IsInternalError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, base, _ := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)
			meta.SetStatusCondition(&plan.transCtx.Cluster.Status.Conditions,
				newFailedApplyResourcesCondition(buildErr))
			plan.cli = interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
				SubResourcePatch: func(
					context.Context,
					client.Client,
					string,
					client.Object,
					client.Patch,
					...client.SubResourcePatchOption,
				) error {
					return test.patchErr
				},
			})

			err := plan.Execute()
			if !test.matchesErr(err) {
				t.Fatalf("execute error = %v, want exact status patch error %v",
					err, test.patchErr)
			}
			readback := &appsv1.Cluster{}
			if err := base.Get(
				context.Background(),
				client.ObjectKeyFromObject(plan.transCtx.Cluster),
				readback,
			); err != nil {
				t.Fatalf("read Cluster: %v", err)
			}
			if got := meta.FindStatusCondition(
				readback.Status.Conditions,
				appsv1.ConditionTypeApplyResources,
			); got != nil {
				t.Fatalf("failure condition persisted after status patch error: %#v",
					got)
			}
		})
	}
}

func TestClusterPlanDelayedRequeueRetainsCompletedPlanEffects(t *testing.T) {
	buildErr := intctrlutil.NewDelayedRequeueError(time.Second, "retry after completed build")
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)

	if err := plan.Execute(); err != nil {
		t.Fatalf("execute completed delayed-requeue plan: %v", err)
	}
	if counters.creates != 1 || counters.statusJSONPatches != 1 {
		t.Fatalf("writes = creates:%d authorityCAS:%d, want 1/1",
			counters.creates, counters.statusJSONPatches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(plan.transCtx.Cluster), readback); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock == nil ||
		readback.Status.TopologyMutationLock.State != appsv1.TopologyMutationLockStateExecuting {
		t.Fatalf("topology lock after delayed requeue = %#v, want retained Executing receipt",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterPlanAuthorityBuildErrorBlocksPartialEffects(t *testing.T) {
	buildErr := fmt.Errorf("%w: competing topology authority",
		errShardingScaleInPlanProgressionUnavailable)
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, buildErr, nil)

	if err := plan.Execute(); !errors.Is(err, errShardingScaleInPlanProgressionUnavailable) {
		t.Fatalf("execute error = %v, want topology authority barrier", err)
	}
	if counters.creates != 0 || counters.statusJSONPatches != 0 {
		t.Fatalf("writes = creates:%d releaseCAS:%d, want 0/0",
			counters.creates, counters.statusJSONPatches)
	}
	effect := &corev1.ConfigMap{}
	getErr := base.Get(context.Background(), client.ObjectKey{
		Namespace: plan.transCtx.Cluster.Namespace,
		Name:      "ordinary-effect",
	}, effect)
	if !apierrors.IsNotFound(getErr) {
		t.Fatalf("ordinary effect read = %v, want NotFound", getErr)
	}
}

func TestClusterPlanFreshAuthorityFailuresBlockPartialEffects(t *testing.T) {
	readErr := errors.New("fresh Cluster read failed")
	sourceErr := errors.New("initial PlanMaterial source failed")

	tests := []struct {
		name     string
		buildErr func(*testing.T, *clusterPlan, client.Client) error
	}{
		{
			name: "fresh Cluster read",
			buildErr: func(
				t *testing.T,
				plan *clusterPlan,
				base client.Client,
			) error {
				t.Helper()
				reader := interceptor.NewClient(
					base.(client.WithWatch),
					interceptor.Funcs{
						Get: func(
							context.Context,
							client.WithWatch,
							client.ObjectKey,
							client.Object,
							...client.GetOption,
						) error {
							return readErr
						},
					},
				)
				_, err := loadFreshClusterForShardingScaleIn(
					context.Background(),
					reader,
					client.ObjectKeyFromObject(plan.transCtx.Cluster),
					plan.transCtx.Cluster.UID,
					plan.transCtx.Cluster.Generation,
				)
				if !errors.Is(err, readErr) {
					t.Fatalf("fresh Cluster error = %v, want wrapped read error", err)
				}
				return err
			},
		},
		{
			name: "initial PlanMaterial source",
			buildErr: func(
				t *testing.T,
				plan *clusterPlan,
				base client.Client,
			) error {
				t.Helper()
				err := addInitialShardingScaleInPlanIntent(
					context.Background(),
					graph.NewDAG(),
					base,
					&clusterTopologyReconcilePlanSourceError{err: sourceErr},
					client.ObjectKeyFromObject(plan.transCtx.Cluster),
					plan.transCtx.Cluster.UID,
					"sharding",
				)
				if !errors.Is(err, sourceErr) {
					t.Fatalf("PlanMaterial source error = %v, want wrapped source error", err)
				}
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, nil)
			plan.buildErr = test.buildErr(t, plan, base)

			if err := plan.Execute(); err == nil {
				t.Fatal("execute accepted an authority failure")
			}
			if counters.creates != 0 {
				t.Fatalf("ordinary effect writes = %d, want 0", counters.creates)
			}
			effect := &corev1.ConfigMap{}
			getErr := base.Get(context.Background(), client.ObjectKey{
				Namespace: plan.transCtx.Cluster.Namespace,
				Name:      "ordinary-effect",
			}, effect)
			if !apierrors.IsNotFound(getErr) {
				t.Fatalf("ordinary effect read = %v, want NotFound", getErr)
			}
		})
	}
}

func TestClusterPlanRequiresExactOrdinaryTopologyAuthorityBeforeEffects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, client.Client, *appsv1.Cluster)
	}{
		{
			name: "lock absent",
			mutate: func(t *testing.T, base client.Client, cluster *appsv1.Cluster) {
				t.Helper()
				cluster.Status.TopologyMutationLock = nil
				if err := base.Status().Update(context.Background(), cluster); err != nil {
					t.Fatalf("remove lock: %v", err)
				}
			},
		},
		{
			name: "owner replaced",
			mutate: func(t *testing.T, base client.Client, cluster *appsv1.Cluster) {
				t.Helper()
				cluster.Status.TopologyMutationLock.OwnerKind =
					appsv1.TopologyMutationLockOwnerShardingScaleIn
				cluster.Status.TopologyMutationLock.OwnerPlanID = "other-plan"
				if err := base.Status().Update(context.Background(), cluster); err != nil {
					t.Fatalf("replace lock owner: %v", err)
				}
			},
		},
		{
			name: "fence replaced",
			mutate: func(t *testing.T, base client.Client, cluster *appsv1.Cluster) {
				t.Helper()
				cluster.Status.TopologyMutationLock.FenceToken =
					strings.Repeat("b", sha256HexLength)
				if err := base.Status().Update(context.Background(), cluster); err != nil {
					t.Fatalf("replace lock fence: %v", err)
				}
			},
		},
		{
			name: "resource version replaced",
			mutate: func(t *testing.T, base client.Client, cluster *appsv1.Cluster) {
				t.Helper()
				if cluster.Labels == nil {
					cluster.Labels = map[string]string{}
				}
				cluster.Labels["test.kubeblocks.io/authority-drift"] = "true"
				if err := base.Update(context.Background(), cluster); err != nil {
					t.Fatalf("replace Cluster resourceVersion: %v", err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, nil)
			fresh := &appsv1.Cluster{}
			if err := base.Get(
				context.Background(),
				client.ObjectKeyFromObject(plan.transCtx.Cluster),
				fresh,
			); err != nil {
				t.Fatalf("read Cluster: %v", err)
			}
			test.mutate(t, base, fresh)

			if err := plan.Execute(); err == nil {
				t.Fatal("execute accepted replaced ordinary topology authority")
			}
			if counters.creates != 0 {
				t.Fatalf("ordinary effect writes = %d, want 0", counters.creates)
			}
			effect := &corev1.ConfigMap{}
			getErr := base.Get(context.Background(), client.ObjectKey{
				Namespace: plan.transCtx.Cluster.Namespace,
				Name:      "ordinary-effect",
			}, effect)
			if !apierrors.IsNotFound(getErr) {
				t.Fatalf("ordinary effect read = %v, want NotFound", getErr)
			}
		})
	}
}

func TestClusterPlanRetainsOrdinaryTopologyLockAfterExecutionError(t *testing.T) {
	createErr := fmt.Errorf("create failed")
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, createErr)

	if err := plan.Execute(); !errors.Is(err, createErr) {
		t.Fatalf("execute error = %v, want create error", err)
	}
	if counters.creates != 1 || counters.statusJSONPatches != 1 {
		t.Fatalf("writes = creates:%d authorityCAS:%d, want 1/1",
			counters.creates, counters.statusJSONPatches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), client.ObjectKeyFromObject(plan.transCtx.Cluster), readback); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock == nil ||
		readback.Status.TopologyMutationLock.State !=
			appsv1.TopologyMutationLockStateExecuting {
		t.Fatalf("topology lock after Execute error = %#v, want retained Executing receipt",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterPlanRotatesPersistedExecutingAuthorityBeforeRetryEffects(t *testing.T) {
	retryErr := errors.New("retry effect failed")
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, retryErr)
	fresh := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(plan.transCtx.Cluster)
	if err := base.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	previousFence := strings.Repeat("d", sha256HexLength)
	fresh.Status.TopologyMutationLock.State =
		appsv1.TopologyMutationLockStateExecuting
	fresh.Status.TopologyMutationLock.FenceToken = previousFence
	if err := base.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("persist prior execution receipt: %v", err)
	}
	if err := base.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("read prior execution receipt: %v", err)
	}
	plan.transCtx.Cluster = fresh.DeepCopy()
	plan.transCtx.OrigCluster = fresh.DeepCopy()
	plan.transCtx.clusterTopologyReconcileLockAuthority =
		fresh.Status.TopologyMutationLock.DeepCopy()
	plan.transCtx.clusterTopologyReconcileLockRelease =
		fresh.Status.TopologyMutationLock.DeepCopy()

	if err := plan.Execute(); !errors.Is(err, retryErr) {
		t.Fatalf("execute retry plan error = %v, want retry effect failure", err)
	}
	if counters.creates != 1 || counters.statusJSONPatches != 1 {
		t.Fatalf("writes = creates:%d authorityCAS:%d, want 1/1",
			counters.creates, counters.statusJSONPatches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(context.Background(), key, readback); err != nil {
		t.Fatalf("read rotated execution receipt: %v", err)
	}
	lock := readback.Status.TopologyMutationLock
	if lock == nil ||
		lock.State != appsv1.TopologyMutationLockStateExecuting ||
		lock.FenceToken == previousFence {
		t.Fatalf("retry execution receipt = %#v, want Executing with a rotated fence",
			lock)
	}
}

func persistClusterTopologyReconcileExecutionLock(
	t *testing.T,
	plan *clusterPlan,
	base client.Client,
) *appsv1.TopologyMutationLockStatus {
	t.Helper()
	fresh := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(plan.transCtx.Cluster),
		fresh,
	); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	executionLock := fresh.Status.TopologyMutationLock.DeepCopy()
	executionLock.State = appsv1.TopologyMutationLockStateExecuting
	executionLock.FenceToken = strings.Repeat("e", sha256HexLength)
	fresh.Status.TopologyMutationLock = executionLock
	if err := base.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("persist execution lock: %v", err)
	}
	plan.transCtx.clusterTopologyReconcileLockAuthority = executionLock.DeepCopy()
	plan.transCtx.clusterTopologyReconcileLockRelease = executionLock.DeepCopy()
	return executionLock
}

func TestClusterTopologyReconcileReleaseRejectsLockReplacement(t *testing.T) {
	plan, base, counters := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	expected := persistClusterTopologyReconcileExecutionLock(t, plan, base)
	fresh := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(plan.transCtx.Cluster)
	if err := base.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	fresh.Status.TopologyMutationLock.FenceToken = strings.Repeat("a", sha256HexLength)
	if err := base.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("replace lock: %v", err)
	}

	err := releaseClusterTopologyReconcileLock(
		context.Background(), base, base, plan.transCtx.Cluster, expected)
	if err == nil {
		t.Fatal("release accepted a replaced lock")
	}
	if counters.statusJSONPatches != 0 {
		t.Fatalf("release CAS attempts = %d, want 0 before identity validation",
			counters.statusJSONPatches)
	}
}

func TestClusterTopologyReconcileReleaseConflictRetainsLock(t *testing.T) {
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	persistClusterTopologyReconcileExecutionLock(t, plan, base)
	conflict := apierrors.NewConflict(
		schema.GroupResource{
			Group:    appsv1.GroupVersion.Group,
			Resource: "clusters",
		},
		plan.transCtx.Cluster.Name,
		errors.New("resourceVersion changed"),
	)
	cli := interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			context.Context,
			client.Client,
			string,
			client.Object,
			client.Patch,
			...client.SubResourcePatchOption,
		) error {
			return conflict
		},
	})

	err := releaseClusterTopologyReconcileLock(
		context.Background(),
		base,
		cli,
		plan.transCtx.Cluster,
		plan.transCtx.clusterTopologyReconcileLockRelease,
	)
	if !apierrors.IsConflict(err) {
		t.Fatalf("release error = %v, want Conflict", err)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(plan.transCtx.Cluster),
		readback,
	); err != nil {
		t.Fatalf("read Cluster after conflict: %v", err)
	}
	if readback.Status.TopologyMutationLock == nil {
		t.Fatal("topology lock was released after release CAS conflict")
	}
}

const sha256HexLength = 64

func TestClusterTopologyReconcileReleaseResolvesReceiptResponseLoss(t *testing.T) {
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	expected := persistClusterTopologyReconcileExecutionLock(t, plan, base)
	responseLost := errors.New("release receipt response lost")
	patches := 0
	cli := interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			patches++
			if err := cli.SubResource(subResourceName).Patch(
				ctx,
				obj,
				patch,
				opts...,
			); err != nil {
				return err
			}
			if patches == 1 {
				return responseLost
			}
			return nil
		},
	})

	if err := releaseClusterTopologyReconcileLock(
		context.Background(),
		base,
		cli,
		plan.transCtx.Cluster,
		expected,
	); err != nil {
		t.Fatalf("resolve release receipt response loss: %v", err)
	}
	if patches != 2 {
		t.Fatalf("status patch attempts = %d, want receipt+cleanup", patches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(plan.transCtx.Cluster),
		readback,
	); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock != nil {
		t.Fatalf("topology lock after resolved response loss = %#v, want nil",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterTopologyReconcileReleaseResolvesCleanupResponseLoss(t *testing.T) {
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	expected := persistClusterTopologyReconcileExecutionLock(t, plan, base)
	responseLost := errors.New("release cleanup response lost")
	patches := 0
	cli := interceptor.NewClient(base.(client.WithWatch), interceptor.Funcs{
		SubResourcePatch: func(
			ctx context.Context,
			cli client.Client,
			subResourceName string,
			obj client.Object,
			patch client.Patch,
			opts ...client.SubResourcePatchOption,
		) error {
			patches++
			if err := cli.SubResource(subResourceName).Patch(
				ctx,
				obj,
				patch,
				opts...,
			); err != nil {
				return err
			}
			if patches == 2 {
				return responseLost
			}
			return nil
		},
	})

	if err := releaseClusterTopologyReconcileLock(
		context.Background(),
		base,
		cli,
		plan.transCtx.Cluster,
		expected,
	); err != nil {
		t.Fatalf("resolve release cleanup response loss: %v", err)
	}
	if patches != 2 {
		t.Fatalf("status patch attempts = %d, want receipt+cleanup", patches)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(plan.transCtx.Cluster),
		readback,
	); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	if readback.Status.TopologyMutationLock != nil {
		t.Fatalf("topology lock after resolved cleanup response loss = %#v, want nil",
			readback.Status.TopologyMutationLock)
	}
}

func TestClusterTopologyReconcileReleaseRequiresExactExecutingReceipt(t *testing.T) {
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	persistClusterTopologyReconcileExecutionLock(t, plan, base)
	fresh := &appsv1.Cluster{}
	key := client.ObjectKeyFromObject(plan.transCtx.Cluster)
	if err := base.Get(context.Background(), key, fresh); err != nil {
		t.Fatalf("read Cluster: %v", err)
	}
	fresh.Status.TopologyMutationLock = nil
	if err := base.Status().Update(context.Background(), fresh); err != nil {
		t.Fatalf("remove lock: %v", err)
	}
	if err := releaseClusterTopologyReconcileLock(
		context.Background(),
		base,
		base,
		plan.transCtx.Cluster,
		plan.transCtx.clusterTopologyReconcileLockRelease,
	); !errors.Is(err, errInvalidClusterTopologyReconcileLock) {
		t.Fatalf("release error = %v, want missing exact Executing receipt rejection", err)
	}
}

func TestClusterTopologyReconcileReleasedReceiptCleanupStopsEffects(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cluster := newClusterTopologyReconcileLockTestCluster()
	lock, err := buildClusterTopologyReconcileLock(cluster)
	if err != nil {
		t.Fatal(err)
	}
	lock.State = appsv1.TopologyMutationLockStateReleased
	cluster.Status.TopologyMutationLock = lock
	base := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(cluster).
		WithObjects(cluster).
		Build()
	persisted := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(cluster),
		persisted,
	); err != nil {
		t.Fatal(err)
	}
	graphCli := model.NewGraphClient(base)
	dag := graph.NewDAG()
	graphCli.Root(
		dag,
		persisted.DeepCopy(),
		persisted.DeepCopy(),
		model.ActionStatusPtr(),
	)
	transCtx := &clusterTransformContext{
		Context:     context.Background(),
		Client:      graphCli,
		APIReader:   base,
		Logger:      logr.Discard(),
		Cluster:     persisted.DeepCopy(),
		OrigCluster: persisted.DeepCopy(),
	}
	effectName := "must-not-run-after-released-receipt"
	buildErr := (graph.TransformerChain{
		&clusterTopologyReconcileAuthorityTransformer{},
		&clusterTopologyReconcilePreAuthorityEffectTestTransformer{
			name: effectName,
		},
	}).ApplyTo(transCtx, dag)
	if buildErr != nil {
		t.Fatalf("build released receipt cleanup: %v", buildErr)
	}
	builder := &clusterPlanBuilder{cli: base, transCtx: transCtx}
	plan := &clusterPlan{
		dag:      dag,
		walkFunc: builder.defaultWalkFuncWithLogging,
		cli:      base,
		transCtx: transCtx,
	}
	if err := plan.Execute(); err != nil {
		t.Fatalf("execute released receipt cleanup: %v", err)
	}
	readback := &appsv1.Cluster{}
	if err := base.Get(
		context.Background(),
		client.ObjectKeyFromObject(cluster),
		readback,
	); err != nil {
		t.Fatal(err)
	}
	if readback.Status.TopologyMutationLock != nil {
		t.Fatalf("released receipt after cleanup = %#v, want nil",
			readback.Status.TopologyMutationLock)
	}
	effect := &corev1.ConfigMap{}
	getErr := base.Get(context.Background(), client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      effectName,
	}, effect)
	if !apierrors.IsNotFound(getErr) {
		t.Fatalf("effect read after released receipt cleanup = %v, want NotFound",
			getErr)
	}
}

func TestClusterTopologyReconcileReleasePreservesNotFound(t *testing.T) {
	plan, base, _ := newClusterTopologyReconcileReleasePlan(t, nil, nil)
	persistClusterTopologyReconcileExecutionLock(t, plan, base)
	if err := base.Delete(context.Background(), plan.transCtx.Cluster.DeepCopy()); err != nil {
		t.Fatalf("delete Cluster: %v", err)
	}
	err := releaseClusterTopologyReconcileLock(
		context.Background(),
		base,
		base,
		plan.transCtx.Cluster,
		plan.transCtx.clusterTopologyReconcileLockRelease,
	)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("release error = %v, want NotFound", err)
	}
}
