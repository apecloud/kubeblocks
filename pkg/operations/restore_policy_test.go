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

package operations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	dptypes "github.com/apecloud/kubeblocks/pkg/dataprotection/types"
)

func TestRestoreActionRejectsSerialBeforeCreatingCluster(t *testing.T) {
	for _, tc := range []struct {
		name string
		spec opsv1alpha1.Restore
	}{
		{name: "field", spec: opsv1alpha1.Restore{VolumeRestorePolicy: "Serial"}},
		{name: "parameter hidden by default field", spec: opsv1alpha1.Restore{VolumeRestorePolicy: "Parallel", Parameters: []dpv1alpha1.ParameterPair{{Name: dptypes.VolumeRestorePolicyParameterKey, Value: "Serial"}}}},
		{name: "duplicate parameter", spec: opsv1alpha1.Restore{Parameters: []dpv1alpha1.ParameterPair{
			{Name: dptypes.VolumeRestorePolicyParameterKey, Value: "Serial"},
			{Name: dptypes.VolumeRestorePolicyParameterKey, Value: "Parallel"},
		}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, appsv1.AddToScheme(scheme))
			cli := fake.NewClientBuilder().WithScheme(scheme).Build()
			ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "restore", Namespace: "default"},
				Spec: opsv1alpha1.OpsRequestSpec{SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{Restore: &tc.spec}}}
			err := (RestoreOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
				&OpsResource{OpsRequest: ops, Recorder: record.NewFakeRecorder(10)})
			require.ErrorContains(t, err, "Serial is no longer supported")
			clusters := &appsv1.ClusterList{}
			require.NoError(t, cli.List(context.Background(), clusters))
			require.Empty(t, clusters.Items)
			phase, _, err := (RestoreOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: context.Background()}, cli,
				&OpsResource{OpsRequest: ops, Recorder: record.NewFakeRecorder(10)})
			require.ErrorContains(t, err, "Serial is no longer supported")
			require.Equal(t, opsv1alpha1.OpsFailedPhase, phase)
		})
	}
}

func TestSerialRestoreOpsWaitsForExistingExecutions(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, dpv1alpha1.AddToScheme(scheme))
	require.NoError(t, batchv1.AddToScheme(scheme))
	ops := &opsv1alpha1.OpsRequest{ObjectMeta: metav1.ObjectMeta{Name: "restore-op", Namespace: "default", UID: "ops-uid"},
		Spec: opsv1alpha1.OpsRequestSpec{ClusterName: "target", SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
			Restore: &opsv1alpha1.Restore{VolumeRestorePolicy: "Serial"},
		}}}
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "target", Namespace: ops.Namespace, UID: "cluster-uid"},
		Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{Parameters: map[string]string{dptypes.VolumeRestorePolicyParameterKey: "Serial"}}}}
	markRestoreClusterWithOps(cluster, ops)
	child := &dpv1alpha1.Restore{ObjectMeta: metav1.ObjectMeta{Name: "execution", Namespace: ops.Namespace,
		Labels: map[string]string{dptypes.ClusterUIDLabelKey: string(cluster.UID), constant.AppInstanceLabelKey: cluster.Name}},
		Status: dpv1alpha1.RestoreStatus{Phase: dpv1alpha1.RestorePhaseRunning}}
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "current-job", Namespace: ops.Namespace}, Status: batchv1.JobStatus{Active: 1}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(child).WithObjects(cluster, child, job).Build()
	res := &OpsResource{OpsRequest: ops, Recorder: record.NewFakeRecorder(10)}
	phase, _, err := (RestoreOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: ctx}, cli, res)
	require.ErrorContains(t, err, "waiting for existing restore executions")
	require.Equal(t, opsv1alpha1.OpsRunningPhase, phase, "do not finish the parent before its existing execution has drained")
	actionErr := (RestoreOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: ctx}, cli, res)
	require.True(t, intctrlutil.IsTargetError(actionErr, intctrlutil.ErrorTypeNeedWaiting),
		"a restart after creating the Cluster but before recording Running must also wait: %v", actionErr)
	require.NoError(t, cli.Get(ctx, client.ObjectKeyFromObject(job), job))
	child.Status.Phase = dpv1alpha1.RestorePhaseFailed
	require.NoError(t, cli.Status().Update(ctx, child))
	phase, _, err = (RestoreOpsHandler{}).ReconcileAction(intctrlutil.RequestCtx{Ctx: ctx}, cli, res)
	require.ErrorContains(t, err, "Serial is no longer supported")
	require.Equal(t, opsv1alpha1.OpsFailedPhase, phase)
	actionErr = (RestoreOpsHandler{}).Action(intctrlutil.RequestCtx{Ctx: ctx}, cli, res)
	require.True(t, intctrlutil.IsTargetError(actionErr, intctrlutil.ErrorTypeFatal))
}
