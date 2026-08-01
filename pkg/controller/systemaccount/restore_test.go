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

func TestReconcileRestoreRequestsKeepsFinalizerOwnership(t *testing.T) {
	registerModelScheme(t)
	testCases := []struct {
		name      string
		owner     client.Object
		finalizer string
	}{
		{
			name: "Component owner",
			owner: &appsv1.Component{
				TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
			},
			finalizer: constant.DBComponentFinalizerName,
		},
		{
			name: "Cluster owner",
			owner: &appsv1.Cluster{
				TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Cluster"},
				ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster", UID: types.UID("cluster-uid")},
			},
			finalizer: constant.DBClusterFinalizerName,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			deletionTime := metav1.Now()
			target := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "default",
					Name:              "cluster-mysql-root",
					DeletionTimestamp: &deletionTime,
					Finalizers:        []string{testCase.finalizer, "another.example.io/finalizer"},
				},
				Immutable: boolPtr(true),
				Data: map[string][]byte{
					constant.AccountNameForSecret:   []byte("root"),
					constant.AccountPasswdForSecret: []byte("old-password"),
				},
			}
			request := newTestRestoreRequest(target.Name)
			require.NoError(t, controllerutil.SetControllerReference(testCase.owner, target, scheme))
			require.NoError(t, controllerutil.SetControllerReference(testCase.owner, request, scheme))
			require.NoError(t, SetRestoreRevision(request))
			graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, request).Build())
			dag := graph.NewDAG()
			graphCli.Root(dag, testCase.owner.DeepCopyObject().(client.Object), testCase.owner, model.ActionStatusPtr())

			handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, testCase.owner, testCase.finalizer)

			require.True(t, intctrlutil.IsRequeueError(err), err)
			require.True(t, handled)
			require.True(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
			vertex := graphCli.FindMatchedVertex(dag, target).(*model.ObjectVertex)
			updated := vertex.Obj.(*corev1.Secret)
			require.NotContains(t, updated.Finalizers, testCase.finalizer)
			require.Contains(t, updated.Finalizers, "another.example.io/finalizer")
		})
	}
}

func TestReconcileRestoreRequestsCreatesTargetBeforeDeletingRequest(t *testing.T) {
	registerModelScheme(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
	}
	request := newTestRestoreRequest("cluster-mysql-root")
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, SetRestoreRevision(request))
	graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build())
	dag := graph.NewDAG()
	graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, owner, constant.DBComponentFinalizerName)

	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, handled)
	objects := graphCli.FindAll(dag, &corev1.Secret{})
	require.Len(t, objects, 1)
	target := objects[0].(*corev1.Secret)
	require.Equal(t, "cluster-mysql-root", target.Name)
	require.True(t, graphCli.IsAction(dag, target, model.ActionCreatePtr()))
	require.Contains(t, target.Finalizers, constant.DBComponentFinalizerName)
	require.Equal(t, []byte("new-password"), target.Data[constant.AccountPasswdForSecret])
	require.NotContains(t, target.Annotations, constant.SystemAccountRestoreTargetAnnotationKey)
	require.NotContains(t, target.Labels, constant.SystemAccountRestoreRequestLabelKey)
	require.Equal(t, request.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey],
		target.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey])
	require.False(t, graphCli.IsAction(dag, request, model.ActionDeletePtr()), "request must survive the target-create commit")
}

func TestReconcileRestoreRequestsDeletesImmutableTargetWithoutReleasingFinalizer(t *testing.T) {
	registerModelScheme(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
	}
	request := newTestRestoreRequest("cluster-mysql-root")
	request.Immutable = nil
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "default",
			Name:       "cluster-mysql-root",
			Finalizers: []string{constant.DBComponentFinalizerName},
		},
		Immutable: boolPtr(true),
		Type:      corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: []byte("old-password"),
		},
	}
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, controllerutil.SetOwnerReference(owner, target, scheme))
	require.NoError(t, SetRestoreRevision(request))
	graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, request).Build())
	dag := graph.NewDAG()
	graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, owner, constant.DBComponentFinalizerName)

	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, handled)
	require.True(t, graphCli.IsAction(dag, target, model.ActionDeletePtr()))
	require.Contains(t, target.Finalizers, constant.DBComponentFinalizerName,
		"the owner releases its finalizer only after deletion is observed")
	require.False(t, graphCli.IsAction(dag, request, model.ActionDeletePtr()))
}

func TestReconcileRestoreRequestsAdoptsTargetBeforeMutatingIt(t *testing.T) {
	registerModelScheme(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
	}
	request := newTestRestoreRequest("cluster-mysql-root")
	request.Immutable = nil
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql-root"},
		Type:       corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: []byte("old-password"),
		},
	}
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, SetRestoreRevision(request))
	graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, request).Build())
	dag := graph.NewDAG()
	graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, owner, constant.DBComponentFinalizerName)

	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, handled)
	require.True(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
	updated := graphCli.FindMatchedVertex(dag, target).(*model.ObjectVertex).Obj.(*corev1.Secret)
	require.Equal(t, []byte("old-password"), updated.Data[constant.AccountPasswdForSecret],
		"the ownership commit must precede the credential mutation")
	require.True(t, metav1.IsControlledBy(updated, owner))
	require.Contains(t, updated.Finalizers, constant.DBComponentFinalizerName)
}

func TestReconcileRestoreRequestsDoesNotReleaseAnotherOwnersFinalizer(t *testing.T) {
	registerModelScheme(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
	}
	otherOwner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "other-mysql", UID: types.UID("other-component-uid")},
	}
	request := newTestRestoreRequest("cluster-mysql-root")
	deletionTime := metav1.Now()
	target := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace:         "default",
		Name:              "cluster-mysql-root",
		DeletionTimestamp: &deletionTime,
		Finalizers:        []string{constant.DBComponentFinalizerName},
	}}
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, controllerutil.SetControllerReference(otherOwner, target, scheme))
	require.NoError(t, SetRestoreRevision(request))
	graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, request).Build())
	dag := graph.NewDAG()
	graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, owner, constant.DBComponentFinalizerName)

	require.ErrorContains(t, err, "is not owned")
	require.True(t, handled)
	require.False(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
	require.Contains(t, target.Finalizers, constant.DBComponentFinalizerName)
}

func TestReconcileRestoreRequestsDeletesRequestOnlyAfterTargetConverges(t *testing.T) {
	registerModelScheme(t)
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Cluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Cluster"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster", UID: types.UID("cluster-uid")},
	}
	request := newTestRestoreRequest("cluster-shard-root")
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "cluster-shard-root",
			Labels:      map[string]string{"role": "root"},
			Annotations: map[string]string{constant.SystemAccountProvisionedAnnotationKey: "true"},
			Finalizers:  []string{constant.DBClusterFinalizerName},
		},
		Immutable: boolPtr(true),
		Type:      corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: []byte("new-password"),
		},
	}
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, controllerutil.SetControllerReference(owner, target, scheme))
	require.NoError(t, SetRestoreRevision(request))
	target.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey] =
		request.Annotations[constant.SystemAccountRestoreRevisionAnnotationKey]
	graphCli := model.NewGraphClient(fake.NewClientBuilder().WithScheme(scheme).WithObjects(target, request).Build())
	dag := graph.NewDAG()
	graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())

	handled, err := ReconcileRestoreRequests(context.Background(), graphCli, dag, owner, constant.DBClusterFinalizerName)

	require.True(t, intctrlutil.IsRequeueError(err), err)
	require.True(t, handled)
	require.True(t, graphCli.IsAction(dag, request, model.ActionDeletePtr()))
	require.False(t, graphCli.IsAction(dag, target, model.ActionUpdatePtr()))
}

func TestValidateRestoreRequestRejectsTamperedPayload(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	owner := &appsv1.Component{
		TypeMeta:   metav1.TypeMeta{APIVersion: appsv1.GroupVersion.String(), Kind: "Component"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "cluster-mysql", UID: types.UID("component-uid")},
	}
	request := newTestRestoreRequest("cluster-mysql-root")
	require.NoError(t, controllerutil.SetControllerReference(owner, request, scheme))
	require.NoError(t, SetRestoreRevision(request))
	request.Labels["admission.example.io/injected"] = "true"
	request.Annotations["admission.example.io/injected"] = "true"
	require.NoError(t, ValidateRestoreRequest(request), "admission metadata is outside the sealed credential payload")
	request.Data[constant.AccountPasswdForSecret] = []byte("tampered-password")

	require.ErrorContains(t, ValidateRestoreRequest(request), "invalid revision")
}

func newTestRestoreRequest(targetName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      RestoreRequestName("default", targetName),
			Labels: map[string]string{
				"role": "root",
				constant.SystemAccountRestoreRequestLabelKey: "true",
			},
			Annotations: map[string]string{
				constant.SystemAccountRestoreTargetAnnotationKey: targetName,
				constant.SystemAccountProvisionedAnnotationKey:   "true",
			},
		},
		Immutable: boolPtr(true),
		Type:      corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			constant.AccountNameForSecret:   []byte("root"),
			constant.AccountPasswdForSecret: []byte("new-password"),
		},
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func registerModelScheme(t *testing.T) {
	t.Helper()
	require.NoError(t, corev1.AddToScheme(model.GetScheme()))
	require.NoError(t, appsv1.AddToScheme(model.GetScheme()))
}
