package dataprotection

import (
	"context"
	"errors"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type statusPatchFailClient struct {
	client.Client
	err error
}

func (c *statusPatchFailClient) Status() client.SubResourceWriter {
	return &statusPatchFailWriter{
		SubResourceWriter: c.Client.Status(),
		err:               c.err,
	}
}

type statusPatchFailWriter struct {
	client.SubResourceWriter
	err error
}

func (w *statusPatchFailWriter) Patch(
	context.Context,
	client.Object,
	client.Patch,
	...client.SubResourcePatchOption,
) error {
	return w.err
}

func TestHandleSyncPVCErrorDoesNotSwallowUnpersistedConditionPatch(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Namespace = "default"
	pvc.Name = "target"
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&corev1.PersistentVolumeClaim{}).
		WithObjects(pvc.DeepCopy()).
		Build()
	patchErr := errors.New("injected PVC status patch failure")
	reconciler := newStatusPatchFailReconciler(baseClient, patchErr)
	reqCtx := testPVCStatusRequestContext()

	current := &corev1.PersistentVolumeClaim{}
	require.NoError(t, baseClient.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), current))
	statusErr := reconciler.UpdatePVCConditions(
		reqCtx,
		current,
		ReasonPopulatingProcessing,
		"waiting for postReady restore",
	)
	require.ErrorIs(t, statusErr, patchErr)
	require.True(t, reconciler.ContainPopulatingCondition(current))

	live := &corev1.PersistentVolumeClaim{}
	require.NoError(t, baseClient.Get(reqCtx.Ctx, client.ObjectKeyFromObject(pvc), live))
	require.Empty(t, live.Status.Conditions)

	result, err := reconciler.handleSyncPVCError(reqCtx, current, statusErr)
	require.ErrorIs(t, err, patchErr,
		"a failed condition patch must not be treated as persisted external progress")
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter)
}

func TestHandleSyncPVCErrorDoesNotSwallowBoundStatusPatchFailure(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Namespace = "default"
	pvc.Name = "target"
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   PersistentVolumeClaimPopulating,
		Status: corev1.ConditionTrue,
	}}
	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&corev1.PersistentVolumeClaim{}).
		WithObjects(pvc.DeepCopy()).
		Build()
	patchErr := errors.New("injected PVC bound status patch failure")
	reconciler := newStatusPatchFailReconciler(baseClient, patchErr)
	reqCtx := testPVCStatusRequestContext()
	pv := &corev1.PersistentVolume{}
	pv.Spec.Capacity = corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse("1Gi"),
	}

	current := pvc.DeepCopy()
	err := reconciler.syncTargetPVCBoundStatus(reqCtx, current, pv)
	require.ErrorIs(t, err, patchErr)

	result, err := reconciler.handleSyncPVCError(reqCtx, current, err)
	require.ErrorIs(t, err, patchErr,
		"a failed bound-status patch must not be hidden by an existing Populating condition")
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter)
}

func TestHandleSyncPVCErrorKeepsExistingExternalErrorBehavior(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{}
	pvc.Status.Conditions = []corev1.PersistentVolumeClaimCondition{{
		Type:   PersistentVolumeClaimPopulating,
		Status: corev1.ConditionTrue,
	}}
	reconciler := &VolumePopulatorReconciler{
		Recorder: record.NewFakeRecorder(10),
	}

	result, err := reconciler.handleSyncPVCError(
		testPVCStatusRequestContext(),
		pvc,
		errors.New("external controller transient error"),
	)

	require.NoError(t, err)
	require.False(t, result.Requeue)
	require.Zero(t, result.RequeueAfter)
}

func newStatusPatchFailReconciler(baseClient client.Client, patchErr error) *VolumePopulatorReconciler {
	return &VolumePopulatorReconciler{
		Client: &statusPatchFailClient{
			Client: baseClient,
			err:    patchErr,
		},
		Recorder: record.NewFakeRecorder(10),
	}
}

func testPVCStatusRequestContext() intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: logr.Discard(),
	}
}
