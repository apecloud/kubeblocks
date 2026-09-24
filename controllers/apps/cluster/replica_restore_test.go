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
	"context"
	"testing"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpctrl "github.com/apecloud/kubeblocks/controllers/dataprotection"
	"github.com/apecloud/kubeblocks/pkg/constant"
	compctrl "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestOrdinaryShardingDoesNotInjectRestore(t *testing.T) {
	c := &appsv1.Cluster{}
	s := &appsv1.ClusterSharding{Name: "mysql", Template: appsv1.ClusterComponentSpec{VolumeClaimTemplates: []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}}}
	require.NotPanics(t, func() { require.NoError(t, applyClusterRestoreIntent(c, nil, []*appsv1.ClusterSharding{s})) })
	require.Nil(t, s.Template.VolumeClaimTemplates[0].Spec.DataSourceRef)
	s.Template.ReplicaRestore = &appsv1.ClusterReplicaRestore{}
	require.ErrorContains(t, applyClusterRestoreIntent(c, nil, []*appsv1.ClusterSharding{s}), "does not support replicaRestore")
}

// Reconcile through normalization, Component update and status persistence. A
// fresh controller must observe restoration even while its Component is not Ready.
func TestClusterReconcileObservesActiveReplicaRestore(t *testing.T) {
	for _, phase := range []appsv1.ComponentPhase{appsv1.UpdatingComponentPhase, appsv1.FailedComponentPhase} {
		t.Run(string(phase), func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			model.AddScheme(appsv1.AddToScheme)
			def := testapps.NewComponentDefinitionFactory("mysql-v1").SetServiceVersion("1.0.0").GetObject()
			def.Status.Phase = appsv1.AvailablePhase
			c := testapps.NewClusterFactory("default", "replica-restore", "").AddComponent("mysql", def.Name).SetReplicas(5).GetObject()
			c.UID = "cluster-uid"
			c.Generation = 2
			spec := &c.Spec.ComponentSpecs[0]
			spec.ServiceVersion = "1.0.0"
			spec.VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}
			spec.ReplicaRestore = &appsv1.ClusterReplicaRestore{Source: appsv1.ClusterRestoreSource{APIGroup: "dataprotection.kubeblocks.io", Kind: "Backup", Name: "backup"}}
			spec.ReplicaRestoreProjection = &appsv1.ReplicaRestoreProjection{StartOrdinal: 3, EndOrdinal: 5, Fingerprint: replicaRestoreFingerprint(c, "mysql", spec.ReplicaRestore, 3, 5)}
			comp, err := compctrl.BuildComponent(c, spec, nil, nil)
			require.NoError(t, err)
			comp.Generation = 2
			comp.Status = appsv1.ComponentStatus{Phase: phase, ObservedGeneration: 2, ReplicaRestore: &appsv1.ReplicaRestoreStatus{Phase: appsv1.ReplicaRestoreFailed, TargetReplicas: 5, RestoredReplicas: 1, ObservedGeneration: 2, Message: "prepareData failed"}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Cluster{}, &appsv1.Component{}).WithObjects(c, comp, def).Build()
			key := client.ObjectKeyFromObject(c)
			for i := 0; i < 2; i++ {
				r := &ClusterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
				_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: key})
				require.NoError(t, err)
				require.NoError(t, cli.Get(ctx, key, c))
				observed := c.Status.ReplicaRestores["mysql"]
				require.Equal(t, appsv1.ReplicaRestoreFailed, observed.Phase)
				require.Equal(t, int32(1), observed.RestoredReplicas)
				require.Equal(t, c.Generation, observed.ObservedGeneration)
				require.Equal(t, "prepareData failed", observed.Message)
			}
		})
	}
}

func TestClusterReconcileRejectedReplicaRestoreEditPreservesAcceptedStatus(t *testing.T) {
	tests := []struct {
		name      string
		edit      func(*appsv1.ClusterComponentSpec)
		wantError string
	}{
		{
			name: "remove active intent",
			edit: func(spec *appsv1.ClusterComponentSpec) {
				spec.ReplicaRestore = nil
			},
			wantError: "cannot remove replicaRestore while restore is active",
		},
		{
			name: "change active source",
			edit: func(spec *appsv1.ClusterComponentSpec) {
				spec.ReplicaRestore.Source.Name = "other-backup"
			},
			wantError: "source and target are immutable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			model.AddScheme(appsv1.AddToScheme)

			def := testapps.NewComponentDefinitionFactory("mysql-v1").SetServiceVersion("1.0.0").GetObject()
			def.Status.Phase = appsv1.AvailablePhase
			c := testapps.NewClusterFactory("default", "replica-restore", "").AddComponent("mysql", def.Name).SetReplicas(5).GetObject()
			c.UID = "cluster-uid"
			c.Generation = 3
			c.Finalizers = []string{dptypes.RestoreProtectionFinalizerName}
			spec := &c.Spec.ComponentSpecs[0]
			spec.ServiceVersion = "1.0.0"
			spec.VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}
			spec.ReplicaRestore = &appsv1.ClusterReplicaRestore{Source: appsv1.ClusterRestoreSource{
				APIGroup: "dataprotection.kubeblocks.io", Kind: "Backup", Name: "backup",
			}}
			spec.ReplicaRestoreProjection = &appsv1.ReplicaRestoreProjection{
				StartOrdinal: 3,
				EndOrdinal:   5,
				Fingerprint:  replicaRestoreFingerprint(c, "mysql", spec.ReplicaRestore, 3, 5),
			}
			comp, err := compctrl.BuildComponent(c, spec, nil, nil)
			require.NoError(t, err)
			comp.Generation = 2
			comp.Status = appsv1.ComponentStatus{
				Phase:              appsv1.UpdatingComponentPhase,
				ObservedGeneration: 2,
				ReplicaRestore: &appsv1.ReplicaRestoreStatus{
					Phase: appsv1.ReplicaRestoreRunning, TargetReplicas: 5, RestoredReplicas: 1,
					ObservedGeneration: 2, Message: "restoring replica 4",
				},
			}
			spec.ReplicaRestoreProjection = nil
			test.edit(spec)
			c.Status.ReplicaRestores = map[string]appsv1.ReplicaRestoreStatus{
				"mysql": {
					Component: "mysql", Phase: appsv1.ReplicaRestoreRunning, TargetReplicas: 5,
					RestoredReplicas: 1, ObservedGeneration: 2, Message: "restoring replica 4",
				},
			}

			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&appsv1.Cluster{}, &appsv1.Component{}).
				WithObjects(c, comp, def).Build()
			r := &ClusterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
			_, err = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(c)})
			require.ErrorContains(t, err, test.wantError)

			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(c), c))
			observed, ok := c.Status.ReplicaRestores["mysql"]
			require.True(t, ok)
			require.Equal(t, appsv1.ReplicaRestoreRunning, observed.Phase)
			require.Equal(t, int32(5), observed.TargetReplicas)
			require.Equal(t, int32(1), observed.RestoredReplicas)
			require.Equal(t, c.Generation, observed.ObservedGeneration)
			require.Equal(t, "restoring replica 4", observed.Message)

			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(comp), comp))
			require.NotNil(t, comp.Spec.ReplicaRestore)

			coordinator := &dpctrl.ClusterRestoreReconciler{Client: cli}
			_, err = coordinator.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(c)})
			require.NoError(t, err)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(c), c))
			require.Contains(t, c.Finalizers, dptypes.RestoreProtectionFinalizerName)
		})
	}
}

func TestReplicaRestoreStatusRejectsStaleObservations(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	c := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default", UID: "uid", Generation: 4}}
	spec := appsv1.ClusterComponentSpec{Name: "mysql", Replicas: 7, ReplicaRestore: &appsv1.ClusterReplicaRestore{Source: appsv1.ClusterRestoreSource{APIGroup: "dataprotection.kubeblocks.io", Kind: "Backup", Name: "next"}}}
	c.Spec.ComponentSpecs = []appsv1.ClusterComponentSpec{spec}
	c.Status.ReplicaRestores = map[string]appsv1.ReplicaRestoreStatus{"mysql": {Phase: appsv1.ReplicaRestoreCompleted, TargetReplicas: 7, ObservedGeneration: 2}}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: constant.GenerateClusterComponentName(c.Name, "mysql"), Namespace: c.Namespace, Generation: 3}, Spec: appsv1.ComponentSpec{Replicas: 7, ReplicaRestore: &appsv1.ReplicaRestoreProjection{StartOrdinal: 5, EndOrdinal: 7, Fingerprint: replicaRestoreFingerprint(c, "mysql", spec.ReplicaRestore, 5, 7)}}, Status: appsv1.ComponentStatus{ReplicaRestore: &appsv1.ReplicaRestoreStatus{Phase: appsv1.ReplicaRestoreCompleted, TargetReplicas: 7, ObservedGeneration: 2}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(comp).WithObjects(comp).Build()
	require.NoError(t, (&clusterStatusTransformer{}).reconcileReplicaRestores(ctx, cli, c))
	require.Equal(t, appsv1.ReplicaRestorePending, c.Status.ReplicaRestores["mysql"].Phase)
	comp.Status.ReplicaRestore.ObservedGeneration = 3
	require.NoError(t, cli.Status().Update(ctx, comp))
	require.NoError(t, (&clusterStatusTransformer{}).reconcileReplicaRestores(ctx, cli, c))
	require.Equal(t, appsv1.ReplicaRestoreCompleted, c.Status.ReplicaRestores["mysql"].Phase)
	c.Spec.ComponentSpecs[0].ReplicaRestore.Source.Name = "changed"
	require.NoError(t, (&clusterStatusTransformer{}).reconcileReplicaRestores(ctx, cli, c))
	require.Equal(t, appsv1.ReplicaRestoreFailed, c.Status.ReplicaRestores["mysql"].Phase)
	c.Spec.ComponentSpecs[0].ReplicaRestore = nil
	require.NoError(t, (&clusterStatusTransformer{}).reconcileReplicaRestores(ctx, cli, c))
	require.Empty(t, c.Status.ReplicaRestores)
}

func TestClusterReconcileOrdinaryShardingWithVolumes(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	model.AddScheme(appsv1.AddToScheme)
	def := testapps.NewComponentDefinitionFactory("mysql-v1").SetServiceVersion("1.0.0").GetObject()
	def.Status.Phase = appsv1.AvailablePhase
	shardDef := testapps.NewShardingDefinitionFactory("mysql-shards", def.Name).GetObject()
	shardDef.Status.Phase = appsv1.AvailablePhase
	c := testapps.NewClusterFactory("default", "ordinary-shards", "").AddSharding("mysql", shardDef.Name, "").SetShards(2).GetObject()
	c.UID = "sharding-cluster"
	c.Generation = 1
	c.Spec.Shardings[0].Template.VolumeClaimTemplates = []appsv1.PersistentVolumeClaimTemplate{{Name: "data"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Cluster{}, &appsv1.Component{}).WithObjects(c, def, shardDef).Build()
	r := &ClusterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(100)}
	_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(c)})
	require.NoError(t, err)
	comps := &appsv1.ComponentList{}
	require.NoError(t, cli.List(ctx, comps))
	require.Len(t, comps.Items, 2)
	for _, comp := range comps.Items {
		require.Len(t, comp.Spec.VolumeClaimTemplates, 1)
		require.Nil(t, comp.Spec.VolumeClaimTemplates[0].Spec.DataSourceRef)
		require.Nil(t, comp.Spec.ReplicaRestore)
	}
}

func TestReplicaRestoreIntentRemovalRequiresCurrentTerminalStatus(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	c := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"}}
	spec := &appsv1.ClusterComponentSpec{Name: "mysql", Replicas: 5}
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Name: "demo-mysql", Namespace: "default", Generation: 3}, Spec: appsv1.ComponentSpec{Replicas: 5, ReplicaRestore: &appsv1.ReplicaRestoreProjection{StartOrdinal: 3, EndOrdinal: 5}}, Status: appsv1.ComponentStatus{ReplicaRestore: &appsv1.ReplicaRestoreStatus{Phase: appsv1.ReplicaRestoreCompleted, ObservedGeneration: 2}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(comp).WithObjects(comp).Build()
	require.ErrorContains(t, applyReplicaRestoreProjection(ctx, cli, c, spec), "cannot remove")
	comp.Status.ReplicaRestore.ObservedGeneration = 3
	require.NoError(t, cli.Status().Update(ctx, comp))
	require.NoError(t, applyReplicaRestoreProjection(ctx, cli, c, spec))
	require.Nil(t, spec.ReplicaRestoreProjection)
}
