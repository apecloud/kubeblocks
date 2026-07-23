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

package component

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/stretchr/testify/require"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	componentctrl "github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/graph"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestAccountAlreadyProvisioned(t *testing.T) {
	transformer := &componentAccountProvisionTransformer{}
	transCtx := &componentTransformContext{
		SynthesizeComponent: &componentctrl.SynthesizedComponent{},
	}

	require.False(t, transformer.accountAlreadyProvisioned(transCtx, &corev1.Secret{}))

	require.True(t, transformer.accountAlreadyProvisioned(transCtx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey: "true",
			},
		},
	}))
}

func TestComponentAccountTransformerFailsClosedForDamagedActiveRestoreRequest(t *testing.T) {
	tests := map[string]func(*corev1.Secret){
		"missing protocol mirror": func(request *corev1.Secret) {
			delete(request.Annotations, systemaccount.RestoreProtocolAnnotationKey)
		},
		"foreign protocol mirror": func(request *corev1.Secret) {
			request.Annotations[systemaccount.RestoreProtocolAnnotationKey] = "foreign"
		},
		"missing request label": func(request *corev1.Secret) {
			delete(request.Labels, constant.SystemAccountRestoreRequestLabelKey)
		},
		"empty phase": func(request *corev1.Secret) {
			delete(request.Annotations, systemaccount.RestoreRequestPhaseAnnotationKey)
		},
		"unknown phase": func(request *corev1.Secret) {
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] = "Unknown"
		},
		"missing protocol finalizer": func(request *corev1.Secret) {
			request.Finalizers = nil
		},
	}
	for name, damage := range tests {
		t.Run(name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, corev1.AddToScheme(scheme))
			require.NoError(t, appsv1.AddToScheme(scheme))
			require.NoError(t, corev1.AddToScheme(model.GetScheme()))
			require.NoError(t, appsv1.AddToScheme(model.GetScheme()))
			root := &appsv1.Cluster{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.GroupVersion.String(),
					Kind:       appsv1.ClusterKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "cluster",
					UID:       "cluster-uid",
				},
				Spec: appsv1.ClusterSpec{Restore: &appsv1.ClusterRestore{
					Source: appsv1.ClusterRestoreSource{
						APIGroup:  "dataprotection.kubeblocks.io",
						Kind:      "Backup",
						Namespace: "default",
						Name:      "backup",
					},
				}},
			}
			owner := &appsv1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.GroupVersion.String(),
					Kind:       appsv1.ComponentKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "cluster-mysql",
					UID:       "component-uid",
				},
			}
			rootIdentity := systemaccount.ObjectIdentity{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ClusterKind,
				Namespace:  root.Namespace,
				Name:       root.Name,
				UID:        root.UID,
			}
			ownerIdentity := systemaccount.ObjectIdentity{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ComponentKind,
				Namespace:  owner.Namespace,
				Name:       owner.Name,
				UID:        owner.UID,
			}
			request, err := systemaccount.BuildRestoreRequest(systemaccount.CredentialIntent{
				Operation: systemaccount.RestoreOperationIdentity{
					Protocol: systemaccount.RestoreProtocolV2,
					Profile:  systemaccount.RestoreProfileInitialCluster,
					Root:     rootIdentity,
					Source: systemaccount.SourceIdentity{
						APIGroup:  "dataprotection.kubeblocks.io",
						Kind:      "Backup",
						Namespace: "default",
						Name:      "backup",
					},
				},
				Target: systemaccount.LogicalTargetIdentity{
					Protocol:  systemaccount.LogicalTargetProtocolV1,
					Namespace: "default",
					Root:      rootIdentity,
					Owner:     ownerIdentity,
					Scope:     systemaccount.SystemAccountScopeComponent,
					Account:   "admin",
				},
				ResolvedSource: systemaccount.ObjectIdentity{
					APIVersion: "dataprotection.kubeblocks.io/v1alpha1",
					Kind:       "Backup",
					Namespace:  "default",
					Name:       "backup",
					UID:        types.UID("backup-uid"),
				},
				Credentials: map[string][]byte{
					constant.AccountNameForSecret:   []byte("admin"),
					constant.AccountPasswdForSecret: []byte("password"),
				},
			})
			require.NoError(t, err)
			request.UID = "request-uid"
			request.ResourceVersion = "1"
			damage(request)

			baseClient := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(root, owner, request).Build()
			graphCli := model.NewGraphClient(baseClient)
			dag := graph.NewDAG()
			graphCli.Root(dag, owner.DeepCopy(), owner, model.ActionStatusPtr())
			transCtx := &componentTransformContext{
				Context:       context.Background(),
				Client:        graphCli,
				APIReader:     graphCli,
				Component:     owner,
				ComponentOrig: owner.DeepCopy(),
			}

			transformErr := (&componentAccountTransformer{}).Transform(transCtx, dag)

			require.True(t, intctrlutil.IsRequeueError(transformErr), transformErr)
			require.Empty(t, graphCli.FindAll(dag, &corev1.Secret{}))
			live := &corev1.Secret{}
			require.NoError(t, baseClient.Get(
				context.Background(), client.ObjectKeyFromObject(request), live))
			require.Equal(t, request.ResourceVersion, live.ResourceVersion)
		})
	}
}
