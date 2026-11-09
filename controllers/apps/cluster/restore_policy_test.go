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

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestClusterRejectsSerialAndPersistsRestoreFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: "default", UID: "cluster-uid"},
		Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{
			Source:     appsv1.ClusterRestoreSource{Kind: "Backup", Name: "backup"},
			Parameters: map[string]string{dptypes.VolumeRestorePolicyParameterKey: "Serial"},
		}}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cluster).WithObjects(cluster).Build()
	r := &ClusterReconciler{Client: cli, Scheme: scheme, Recorder: record.NewFakeRecorder(20)}
	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(cluster)})
	require.NoError(t, err)
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster))
	condition := meta.FindStatusCondition(cluster.Status.Conditions, appsv1.ConditionTypeRestore)
	require.NotNil(t, condition)
	require.Equal(t, metav1.ConditionFalse, condition.Status)
	require.Contains(t, condition.Message, "Serial is no longer supported")
	components := &appsv1.ComponentList{}
	require.NoError(t, cli.List(context.Background(), components))
	require.Empty(t, components.Items)
}
