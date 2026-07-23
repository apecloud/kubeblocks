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

package systemaccount

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestRestoreRequestPendingClaimsBeforeTargetMutation(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, graphCli.IsAction(dag, fixture.request, model.ActionUpdatePtr()))
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseClaimed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Len(t, graphCli.FindAll(dag, &corev1.Secret{}), 1,
		"claim round must not mutate the target")
}

func TestRestoreRequestPendingFailsBeforeClaimWhenOperationTerminal(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.root.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: fixture.root.Generation,
	}}
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, OperationTerminalReason,
		updated.Annotations[RestoreRequestReasonAnnotationKey])
	require.Len(t, graphCli.FindAll(dag, &corev1.Secret{}), 1)
}

func TestRestoreRequestPendingFailsBeforeClaimWhenTargetOwnerUnavailable(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	replacementOwner := fixture.owner.DeepCopy()
	replacementOwner.UID = types.UID("replacement-component-uid")
	fixture.objects = []client.Object{fixture.root, replacementOwner, fixture.request}
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, TargetOwnerUnavailableReason,
		updated.Annotations[RestoreRequestReasonAnnotationKey])
	require.Len(t, graphCli.FindAll(dag, &corev1.Secret{}), 1)
}

func TestRestoreRequestUsesLivePhaseBeforePlanningMutation(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	liveRequest := fixture.request.DeepCopy()
	liveRequest.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseFailed)
	liveRequest.Annotations[RestoreRequestReasonAnnotationKey] = OperationTerminalReason
	authorityReader := fake.NewClientBuilder().WithScheme(fixture.scheme).
		WithObjects(fixture.root, fixture.owner, liveRequest).Build()
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, authorityReader,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}),
		"a stale cached Pending phase must not overwrite the live Failed request or create a target")
}

func TestClaimedRestoreRequestCreatesAppsOwnedTarget(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	target := findTargetVertex(t, graphCli, dag, fixture.request)
	require.True(t, graphCli.IsAction(dag, target, model.ActionCreatePtr()))
	require.Equal(t, []byte("new-password"), target.Data[constant.AccountPasswdForSecret])
	require.Equal(t, "true", target.Annotations[constant.SystemAccountProvisionedAnnotationKey])
	require.Equal(t, fixture.request.Name, target.Annotations[RestoreRequestNameAnnotationKey])
	require.Equal(t, string(fixture.request.UID), target.Annotations[RestoreRequestUIDAnnotationKey])
	revision, revisionErr := TargetCommitRevision(target, constant.DBComponentFinalizerName)
	require.NoError(t, revisionErr)
	require.NotEmpty(t, target.Annotations[TargetCommitRevisionAnnotationKey])
	require.Equal(t, revision, target.Annotations[TargetCommitRevisionAnnotationKey])
	require.Contains(t, target.Finalizers, constant.DBComponentFinalizerName)
	require.NotContains(t, target.Labels, constant.SystemAccountRestoreRequestLabelKey)
}

func TestClaimedRestoreRequestDoesNotMutateTargetWhenOwnerUnavailable(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	replacementOwner := fixture.owner.DeepCopy()
	replacementOwner.UID = types.UID("replacement-component-uid")
	fixture.objects = []client.Object{fixture.root, replacementOwner, fixture.request}
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}),
		"unavailable target owner must prevent request and target mutations in the Apps plan")
}

func TestClaimedRestoreRequestFailsBeforeTargetMutationWhenOperationTerminal(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	fixture.root.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: fixture.root.Generation,
	}}
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	vertex := graphCli.FindMatchedVertex(dag, fixture.request)
	require.NotNil(t, vertex, "terminal operation was not projected to the request")
	updated := vertex.(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, OperationTerminalReason,
		updated.Annotations[RestoreRequestReasonAnnotationKey])
	require.Len(t, graphCli.FindAll(dag, &corev1.Secret{}), 1,
		"terminal operation must not mutate a non-exact target")
}

func TestClaimedRestoreRequestLeavesExactTargetForTerminalLifecycleRepair(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	fixture.root.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: fixture.root.Generation,
	}}
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	target.UID = types.UID("target-uid")
	target.ResourceVersion = "7"
	applyTargetReceipt(target, fixture.request)
	revision, err := TargetCommitRevision(target, constant.DBComponentFinalizerName)
	require.NoError(t, err)
	target.Annotations[TargetCommitRevisionAnnotationKey] = revision
	fixture.objects = append(fixture.objects, target)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.False(t, graphCli.IsAction(dag, fixture.request, model.ActionUpdatePtr()))
	require.False(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
	require.Equal(t, revision, target.Annotations[TargetCommitRevisionAnnotationKey])
}

func TestClaimedRestoreRequestLeavesExactTargetForLifecycleCommit(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	target.UID = types.UID("target-uid")
	target.ResourceVersion = "7"
	applyTargetReceipt(target, fixture.request)
	revision, err := TargetCommitRevision(target, constant.DBComponentFinalizerName)
	require.NoError(t, err)
	target.Annotations[TargetCommitRevisionAnnotationKey] = revision
	fixture.objects = append(fixture.objects, target)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.False(t, graphCli.IsAction(dag, fixture.request, model.ActionUpdatePtr()),
		"the target transformer must not commit before the lifecycle controller rechecks the operation")
	require.Equal(t, string(RestoreRequestPhaseClaimed),
		fixture.request.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, revision, target.Annotations[TargetCommitRevisionAnnotationKey])
}

func TestClaimedRestoreRequestUsesAuthorityReaderForExactTarget(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	target.UID = types.UID("target-uid")
	target.ResourceVersion = "7"
	applyTargetReceipt(target, fixture.request)
	revision, err := TargetCommitRevision(target, constant.DBComponentFinalizerName)
	require.NoError(t, err)
	target.Annotations[TargetCommitRevisionAnnotationKey] = revision
	authorityReader := fake.NewClientBuilder().WithScheme(fixture.scheme).
		WithObjects(fixture.root, fixture.owner, fixture.request, target).Build()
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, authorityReader,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}),
		"an exact strong-read target must not be recreated from a stale cache miss")
}

func TestClaimedRestoreRequestDeletesImmutableTargetWithObservedUID(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	immutable := true
	target.Immutable = &immutable
	target.UID = types.UID("old-target-uid")
	target.ResourceVersion = "8"
	target.Data[constant.AccountPasswdForSecret] = []byte("old-password")
	fixture.objects = append(fixture.objects, target)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, graphCli.IsAction(dag, target, model.ActionDeletePtr()))
	vertex := graphCli.FindMatchedVertex(dag, target).(*model.ObjectVertex)
	require.NotNil(t, vertex.DeletePreconditions)
	require.NotNil(t, vertex.DeletePreconditions.UID)
	require.Equal(t, target.UID, *vertex.DeletePreconditions.UID)
	require.False(t, graphCli.IsAction(dag, fixture.request, model.ActionUpdatePtr()))
}

func TestClaimedRestoreRequestPreservesUnownedTargetMetadata(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	target.UID = "target-uid"
	target.ResourceVersion = "8"
	target.Data[constant.AccountPasswdForSecret] = []byte("old-password")
	target.Labels["external-label"] = "keep"
	target.Annotations["external-annotation"] = "keep"
	target.Finalizers = append(target.Finalizers, "external.example.io/protection")
	notController := false
	target.OwnerReferences = append(target.OwnerReferences, metav1.OwnerReference{
		APIVersion: "example.io/v1",
		Kind:       "Observer",
		Name:       "observer",
		UID:        "observer-uid",
		Controller: &notController,
	})
	fixture.objects = append(fixture.objects, target)
	graphCli, dag := fixture.graph()

	handled, reconcileErr := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(reconcileErr), reconcileErr)
	vertex := graphCli.FindMatchedVertex(dag, target).(*model.ObjectVertex)
	require.Equal(t, model.ActionUpdatePtr(), vertex.Action)
	updated := vertex.Obj.(*corev1.Secret)
	require.Equal(t, "keep", updated.Labels["external-label"])
	require.Equal(t, "keep", updated.Annotations["external-annotation"])
	require.Contains(t, updated.Finalizers, "external.example.io/protection")
	require.Contains(t, updated.OwnerReferences, target.OwnerReferences[1])
	require.Equal(t, []byte("new-password"), updated.Data[constant.AccountPasswdForSecret])
}

func TestDeletingRequestFailsWithoutTargetMutation(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	now := metav1.Now()
	fixture.request.DeletionTimestamp = &now
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, "RequestDeletionRequested",
		updated.Annotations[RestoreRequestReasonAnnotationKey])
	require.Len(t, graphCli.FindAll(dag, &corev1.Secret{}), 1)
}

func TestUnavailableTargetSemanticFailsPendingRequest(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	graphCli, dag := fixture.graph()

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName,
		func(CredentialIntent) (*corev1.Secret, error) {
			return nil, context.Canceled
		})

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(err), err)
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, "TargetSemanticUnavailable",
		updated.Annotations[RestoreRequestReasonAnnotationKey])
}

func TestClaimedRestoreRequestRejectsMatchingNonControllerOwnerReference(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	fixture.request.Annotations[RestoreRequestPhaseAnnotationKey] = string(RestoreRequestPhaseClaimed)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	notController := false
	target.OwnerReferences[0].Controller = &notController
	controller := true
	target.OwnerReferences = append(target.OwnerReferences, metav1.OwnerReference{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ComponentKind,
		Name:       "foreign-component",
		UID:        "foreign-component-uid",
		Controller: &controller,
	})
	fixture.objects = append(fixture.objects, target)
	graphCli, dag := fixture.graph()

	handled, reconcileErr := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.True(t, handled)
	require.True(t, intctrlutil.IsRequeueError(reconcileErr), reconcileErr)
	updated := graphCli.FindMatchedVertex(dag, fixture.request).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, string(RestoreRequestPhaseFailed),
		updated.Annotations[RestoreRequestPhaseAnnotationKey])
	require.Equal(t, TargetOwnerUnavailableReason,
		updated.Annotations[RestoreRequestReasonAnnotationKey])
	require.False(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
}

func TestReconcileRestoreRequestsIgnoresDamagedForeignRequest(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	foreignIntent := fixture.intent
	foreignIntent.Target.Owner = ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ComponentKind,
		Namespace:  fixture.owner.Namespace,
		Name:       "foreign-component",
		UID:        "foreign-component-uid",
	}
	foreign, err := BuildRestoreRequest(foreignIntent)
	require.NoError(t, err)
	foreign.UID = "foreign-request-uid"
	foreign.ResourceVersion = "1"
	foreign.Annotations[LogicalTargetDigestAnnotationKey] = "damaged"
	fixture.objects = []client.Object{foreign}
	graphCli, dag := fixture.graph()

	handled, reconcileErr := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.NoError(t, reconcileErr)
	require.False(t, handled)
	require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}))
}

func TestReconcileRestoreRequestsFailsClosedForOwnedDamagedRequest(t *testing.T) {
	tests := map[string]func(*corev1.Secret){
		"missing protocol mirror": func(request *corev1.Secret) {
			delete(request.Annotations, RestoreProtocolAnnotationKey)
		},
		"foreign protocol mirror": func(request *corev1.Secret) {
			request.Annotations[RestoreProtocolAnnotationKey] = "foreign"
		},
		"missing request label": func(request *corev1.Secret) {
			delete(request.Labels, constant.SystemAccountRestoreRequestLabelKey)
		},
		"empty phase": func(request *corev1.Secret) {
			delete(request.Annotations, RestoreRequestPhaseAnnotationKey)
		},
		"unknown phase": func(request *corev1.Secret) {
			request.Annotations[RestoreRequestPhaseAnnotationKey] = "Unknown"
		},
		"missing protocol finalizer": func(request *corev1.Secret) {
			request.Finalizers = nil
		},
	}
	for name, damage := range tests {
		t.Run(name, func(t *testing.T) {
			fixture := newRestoreStateFixture(t)
			damage(fixture.request)
			graphCli, dag := fixture.graph()
			builderCalled := false

			handled, reconcileErr := ReconcileRestoreRequests(
				context.Background(), graphCli, dag, graphCli,
				fixture.owner, constant.DBComponentFinalizerName,
				func(intent CredentialIntent) (*corev1.Secret, error) {
					builderCalled = true
					return fixture.targetBuilder(intent)
				})

			require.True(t, handled)
			require.True(t, intctrlutil.IsRequeueError(reconcileErr), reconcileErr)
			require.False(t, builderCalled)
			require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}),
				"damaged request must block normal target mutation without planning its own update")
		})
	}
}

func TestReconcileRestoreRequestsDoesNotTreatTargetReceiptAsRequest(t *testing.T) {
	fixture := newRestoreStateFixture(t)
	target, err := fixture.targetBuilder(fixture.intent)
	require.NoError(t, err)
	target.Annotations[RestoreProtocolAnnotationKey] = RestoreProtocolV2
	target.Annotations[RestoreRequestNameAnnotationKey] = fixture.request.Name
	target.Annotations[RestoreRequestUIDAnnotationKey] = string(fixture.request.UID)
	fixture.objects = []client.Object{fixture.root, fixture.owner, target}
	graphCli, dag := fixture.graph()

	handled, reconcileErr := ReconcileRestoreRequests(context.Background(), graphCli, dag, graphCli,
		fixture.owner, constant.DBComponentFinalizerName, fixture.targetBuilder)

	require.NoError(t, reconcileErr)
	require.False(t, handled)
	require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}))
}

type restoreStateFixture struct {
	t       *testing.T
	scheme  *runtime.Scheme
	root    *appsv1.Cluster
	owner   *appsv1.Component
	intent  CredentialIntent
	request *corev1.Secret
	objects []client.Object
}

func newRestoreStateFixture(t *testing.T) *restoreStateFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(model.GetScheme()))
	require.NoError(t, appsv1.AddToScheme(model.GetScheme()))
	root := &appsv1.Cluster{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster",
			UID:       types.UID("cluster-uid"),
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup:  "dataprotection.kubeblocks.io",
					Kind:      "Backup",
					Namespace: "backup",
					Name:      "backup-1",
				},
				Parameters: map[string]string{"restore": "all"},
			},
		},
	}
	owner := &appsv1.Component{
		TypeMeta: metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "cluster-mysql",
			UID:       types.UID("component-uid"),
		},
	}
	intent := testCredentialIntent()
	request, err := BuildRestoreRequest(intent)
	require.NoError(t, err)
	request.UID = types.UID("request-uid")
	request.ResourceVersion = "3"
	return &restoreStateFixture{
		t:       t,
		scheme:  scheme,
		root:    root,
		owner:   owner,
		intent:  intent,
		request: request,
		objects: []client.Object{root, owner, request},
	}
}

func (f *restoreStateFixture) graph() (model.GraphClient, *graph.DAG) {
	f.t.Helper()
	cli := fake.NewClientBuilder().WithScheme(f.scheme).WithObjects(f.objects...).Build()
	graphCli := model.NewGraphClient(cli)
	dag := graph.NewDAG()
	graphCli.Root(dag, f.owner.DeepCopy(), f.owner, model.ActionStatusPtr())
	return graphCli, dag
}

func (f *restoreStateFixture) targetBuilder(intent CredentialIntent) (*corev1.Secret, error) {
	f.t.Helper()
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   intent.Target.Namespace,
			Name:        "cluster-mysql-root",
			Labels:      map[string]string{"role": "root"},
			Annotations: map[string]string{"semantic": "live"},
			Finalizers:  []string{constant.DBComponentFinalizerName},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   append([]byte(nil), intent.Credentials[constant.AccountNameForSecret]...),
			constant.AccountPasswdForSecret: append([]byte(nil), intent.Credentials[constant.AccountPasswdForSecret]...),
		},
	}
	require.NoError(f.t, controllerutil.SetControllerReference(f.owner, target, f.scheme))
	return target, nil
}

func findTargetVertex(t *testing.T, graphCli model.GraphClient, dag *graph.DAG, request *corev1.Secret) *corev1.Secret {
	t.Helper()
	for _, object := range graphCli.FindAll(dag, &corev1.Secret{}) {
		secret := object.(*corev1.Secret)
		if secret.Name != request.Name {
			return secret
		}
	}
	t.Fatal("target vertex not found")
	return nil
}
