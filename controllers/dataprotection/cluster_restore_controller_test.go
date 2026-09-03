/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

This file is part of KubeBlocks project

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

package dataprotection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestClusterRestoreProtectionLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name      string
		deleting  bool
		protected bool
		status    metav1.ConditionStatus
		resource  string
		wantKeep  bool
		wantWait  bool
	}{
		{"initial intent", false, false, metav1.ConditionUnknown, "", true, false},
		{"deletion without protection", true, false, metav1.ConditionUnknown, "target", false, false},
		{"failed restore", false, true, metav1.ConditionFalse, "", true, false},
		{"successful restore", false, true, metav1.ConditionTrue, "completed restore", false, false},
		{"target still protected", false, true, metav1.ConditionTrue, "target", true, false},
		{"helper remains", false, true, metav1.ConditionTrue, "helper", true, false},
		{"execution still running", false, true, metav1.ConditionTrue, "running restore", true, false},
		{"deletion waits for target", true, true, metav1.ConditionTrue, "target", true, true},
		{"deletion waits for helper", true, true, metav1.ConditionTrue, "helper", true, true},
		{"deletion waits for failed restore", true, true, metav1.ConditionTrue, "failed restore", true, true},
		{"deletion waits for completed restore", true, true, metav1.ConditionTrue, "completed restore", true, true},
		{"deletion cleanup finished", true, true, metav1.ConditionTrue, "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme, cluster, _, _, target := parentRestoreObjects(t)
			cluster.Finalizers = []string{"example.io/app-owner"}
			if tc.protected {
				cluster.Finalizers = append(cluster.Finalizers, dptypes.RestoreProtectionFinalizerName)
			}
			if tc.deleting {
				now := metav1.Now()
				cluster.DeletionTimestamp = &now
			}
			cluster.Status.Conditions = []metav1.Condition{{
				Type: appsv1.ConditionTypeRestore, Status: tc.status,
			}}
			var resource client.Object
			switch tc.resource {
			case "target":
				target.Finalizers = []string{dptypes.DataProtectionFinalizerName}
				resource = target
			case "helper":
				resource = restoreHelperForTarget(target, cluster)
			case "running restore", "failed restore", "completed restore":
				restore := executionRestoreForTarget(target, cluster)
				switch tc.resource {
				case "failed restore":
					restore.Status.Phase = dpv1alpha1.RestorePhaseFailed
				case "completed restore":
					restore.Status.Phase = dpv1alpha1.RestorePhaseCompleted
				default:
					restore.Status.Phase = dpv1alpha1.RestorePhaseRunning
				}
				restore.Finalizers = []string{dptypes.DataProtectionFinalizerName}
				resource = restore
			}
			objects := []client.Object{cluster}
			if resource != nil {
				objects = append(objects, resource)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
			if resource != nil {
				require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(resource), resource))
			}
			reconciler := &ClusterRestoreReconciler{Client: cli}

			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
			require.NoError(t, err)
			require.Equal(t, tc.wantWait, result.RequeueAfter > 0)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster))
			expected := []string{"example.io/app-owner"}
			if tc.wantKeep {
				expected = append(expected, dptypes.RestoreProtectionFinalizerName)
			}
			require.ElementsMatch(t, expected, cluster.Finalizers)
			if resource != nil {
				current := resource.DeepCopyObject().(client.Object)
				require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(resource), current))
				require.Equal(t, resource, current, "ClusterRestore must not delete or mutate another controller's resource")
			}
		})
	}
}

func TestClusterRestoreControllerIgnoresResourcesWithoutExactClusterUID(t *testing.T) {
	for _, uid := range []string{"", "another-cluster-uid"} {
		t.Run("uid="+uid, func(t *testing.T) {
			ctx := context.Background()
			scheme, cluster, _, _, target := parentRestoreObjects(t)
			now := metav1.Now()
			cluster.DeletionTimestamp = &now
			cluster.Finalizers = append(cluster.Finalizers, "example.io/app-owner")
			target.Finalizers = []string{dptypes.DataProtectionFinalizerName}
			helper := restoreHelperForTarget(target, cluster)
			restore := executionRestoreForTarget(target, cluster)
			for _, obj := range []client.Object{target, helper, restore} {
				obj.GetLabels()[dptypes.ClusterUIDLabelKey] = uid
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, target, helper, restore).Build()
			reconciler := &ClusterRestoreReconciler{Client: cli}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
			require.NoError(t, err)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(cluster), cluster))
			require.Equal(t, []string{"example.io/app-owner"}, cluster.Finalizers)
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(target), &corev1.PersistentVolumeClaim{}))
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(helper), &corev1.PersistentVolumeClaim{}))
			require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(restore), &dpv1alpha1.Restore{}))
		})
	}
}
