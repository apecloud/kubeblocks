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

package operations

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	opsv1alpha1 "github.com/apecloud/kubeblocks/apis/operations/v1alpha1"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type failClusterUpdateClient struct {
	client.Client
	fail bool
}

func (c *failClusterUpdateClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if _, ok := obj.(*appsv1.Cluster); ok && c.fail {
		c.fail = false
		return apierrors.NewConflict(schema.GroupResource{Group: appsv1.GroupVersion.Group, Resource: "clusters"}, obj.GetName(), errors.New("conflict"))
	}
	return c.Client.Update(ctx, obj, opts...)
}

type failStatusPatchClient struct {
	client.Client
	fail bool
}

func (c *failStatusPatchClient) Status() client.StatusWriter {
	return &failStatusPatchWriter{StatusWriter: c.Client.Status(), client: c}
}

type failStatusPatchWriter struct {
	client.StatusWriter
	client *failStatusPatchClient
}

func (w *failStatusPatchWriter) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.SubResourcePatchOption) error {
	if w.client.fail {
		w.client.fail = false
		return apierrors.NewConflict(schema.GroupResource{Group: opsv1alpha1.GroupVersion.Group, Resource: "opsrequests/status"}, obj.GetName(), errors.New("conflict"))
	}
	return w.StatusWriter.Patch(ctx, obj, patch, opts...)
}

func TestCheckAndHandleOpsTimeoutRollsBackHorizontalScaling(t *testing.T) {
	opsRes, cli := newTimedOutHorizontalScalingOpsResource(t)
	behaviour := OpsBehaviour{
		CancelFunc:        horizontalScalingOpsHandler{}.Cancel,
		RollbackOnTimeout: true,
	}

	requeueAfter, err := GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, behaviour, 0)
	require.NoError(t, err)
	require.Zero(t, requeueAfter)

	cluster := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.Cluster), cluster))
	require.Equal(t, int32(3), cluster.Spec.ComponentSpecs[0].Replicas)

	opsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.OpsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsAbortedPhase, opsRequest.Status.Phase)
	require.Condition(t, func() bool {
		for _, condition := range opsRequest.Status.Conditions {
			if strings.Contains(condition.Message, "original desired configuration has been restored") {
				return true
			}
		}
		return false
	})
}

func TestCheckAndHandleOpsTimeoutRetriesRollbackBeforeAborting(t *testing.T) {
	opsRes, baseClient := newTimedOutHorizontalScalingOpsResource(t)
	cli := &failClusterUpdateClient{Client: baseClient, fail: true}
	behaviour := OpsBehaviour{
		CancelFunc:        horizontalScalingOpsHandler{}.Cancel,
		RollbackOnTimeout: true,
	}

	_, err := GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, behaviour, 0)
	require.True(t, apierrors.IsConflict(err))

	cluster := &appsv1.Cluster{}
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRes.Cluster), cluster))
	require.Equal(t, int32(4), cluster.Spec.ComponentSpecs[0].Replicas)
	opsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRes.OpsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsRunningPhase, opsRequest.Status.Phase)

	retryRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest, Recorder: record.NewFakeRecorder(10)}
	_, err = GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, retryRes, behaviour, 0)
	require.NoError(t, err)

	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster))
	require.Equal(t, int32(3), cluster.Spec.ComponentSpecs[0].Replicas)
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsAbortedPhase, opsRequest.Status.Phase)
}

func TestCheckAndHandleOpsTimeoutRetriesStatusCommitAfterRollback(t *testing.T) {
	opsRes, baseClient := newTimedOutHorizontalScalingOpsResource(t)
	cli := &failStatusPatchClient{Client: baseClient, fail: true}
	behaviour := OpsBehaviour{
		CancelFunc:        horizontalScalingOpsHandler{}.Cancel,
		RollbackOnTimeout: true,
	}

	_, err := GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, behaviour, 0)
	require.True(t, apierrors.IsConflict(err))

	cluster := &appsv1.Cluster{}
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRes.Cluster), cluster))
	require.Equal(t, int32(3), cluster.Spec.ComponentSpecs[0].Replicas)
	opsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRes.OpsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsRunningPhase, opsRequest.Status.Phase)

	retryRes := &OpsResource{Cluster: cluster, OpsRequest: opsRequest, Recorder: record.NewFakeRecorder(10)}
	_, err = GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, retryRes, behaviour, 0)
	require.NoError(t, err)

	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(cluster), cluster))
	require.Equal(t, int32(3), cluster.Spec.ComponentSpecs[0].Replicas)
	require.NoError(t, baseClient.Get(context.Background(), client.ObjectKeyFromObject(opsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsAbortedPhase, opsRequest.Status.Phase)
}

func TestCheckAndHandleOpsTimeoutPreservesUnsupportedRollbackBehavior(t *testing.T) {
	opsRes, cli := newTimedOutHorizontalScalingOpsResource(t)
	behaviour := OpsBehaviour{
		CancelFunc: func(intctrlutil.RequestCtx, client.Client, *OpsResource) error {
			return intctrlutil.NewErrorf(intctrlutil.ErrorIgnoreCancel, "rollback is not supported")
		},
		RollbackOnTimeout: true,
	}

	_, err := GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, behaviour, 0)
	require.NoError(t, err)

	cluster := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.Cluster), cluster))
	require.Equal(t, int32(4), cluster.Spec.ComponentSpecs[0].Replicas)
	opsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.OpsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsAbortedPhase, opsRequest.Status.Phase)
	for _, condition := range opsRequest.Status.Conditions {
		require.NotContains(t, condition.Message, "original desired configuration has been restored")
	}
}

func TestCheckAndHandleOpsTimeoutDoesNotRollbackByDefault(t *testing.T) {
	opsRes, cli := newTimedOutHorizontalScalingOpsResource(t)
	called := false
	behaviour := OpsBehaviour{
		CancelFunc: func(intctrlutil.RequestCtx, client.Client, *OpsResource) error {
			called = true
			return nil
		},
	}

	_, err := GetOpsManager().checkAndHandleOpsTimeout(
		intctrlutil.RequestCtx{Ctx: context.Background()}, cli, opsRes, behaviour, 0)
	require.NoError(t, err)
	require.False(t, called)

	cluster := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.Cluster), cluster))
	require.Equal(t, int32(4), cluster.Spec.ComponentSpecs[0].Replicas)
	opsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRes.OpsRequest), opsRequest))
	require.Equal(t, opsv1alpha1.OpsAbortedPhase, opsRequest.Status.Phase)
}

func newTimedOutHorizontalScalingOpsResource(t *testing.T) (*OpsResource, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, opsv1alpha1.AddToScheme(scheme))

	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"},
		Spec: appsv1.ClusterSpec{ComponentSpecs: []appsv1.ClusterComponentSpec{{
			Name:     "component",
			Replicas: 4,
		}}},
	}
	timeoutSeconds := int32(1)
	opsRequest := &opsv1alpha1.OpsRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "ops", Namespace: "default"},
		Spec: opsv1alpha1.OpsRequestSpec{
			ClusterName:    cluster.Name,
			Type:           opsv1alpha1.HorizontalScalingType,
			TimeoutSeconds: &timeoutSeconds,
			SpecificOpsRequest: opsv1alpha1.SpecificOpsRequest{
				HorizontalScalingList: []opsv1alpha1.HorizontalScaling{{
					ComponentOps: opsv1alpha1.ComponentOps{ComponentName: "component"},
					ScaleOut:     &opsv1alpha1.ScaleOut{ReplicaChanger: opsv1alpha1.ReplicaChanger{ReplicaChanges: pointer.Int32(1)}},
				}},
			},
		},
		Status: opsv1alpha1.OpsRequestStatus{
			Phase:          opsv1alpha1.OpsRunningPhase,
			StartTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Second)),
			LastConfiguration: opsv1alpha1.LastConfiguration{Components: map[string]opsv1alpha1.LastComponentConfiguration{
				"component": {Replicas: pointer.Int32(3)},
			}},
		},
	}
	cli := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&opsv1alpha1.OpsRequest{}).
		WithObjects(cluster, opsRequest).
		Build()

	storedCluster := &appsv1.Cluster{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(cluster), storedCluster))
	storedOpsRequest := &opsv1alpha1.OpsRequest{}
	require.NoError(t, cli.Get(context.Background(), client.ObjectKeyFromObject(opsRequest), storedOpsRequest))
	return &OpsResource{
		Cluster:    storedCluster,
		OpsRequest: storedOpsRequest,
		Recorder:   record.NewFakeRecorder(10),
	}, cli
}
