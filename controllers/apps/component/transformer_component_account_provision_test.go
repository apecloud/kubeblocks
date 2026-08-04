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
	"k8s.io/utils/ptr"
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

func TestComponentRestoreStrongReadsLiveDefinition(t *testing.T) {
	tests := []struct {
		name          string
		phase         systemaccount.RestoreRequestPhase
		liveDef       bool
		accountExists bool
		disabled      bool
		expectTarget  bool
	}{
		{
			name:  "pending/missing definition",
			phase: systemaccount.RestoreRequestPhasePending,
		},
		{
			name:  "claimed/missing definition",
			phase: systemaccount.RestoreRequestPhaseClaimed,
		},
		{
			name:    "pending/missing account",
			phase:   systemaccount.RestoreRequestPhasePending,
			liveDef: true,
		},
		{
			name:    "claimed/missing account",
			phase:   systemaccount.RestoreRequestPhaseClaimed,
			liveDef: true,
		},
		{
			name:          "pending/disabled account",
			phase:         systemaccount.RestoreRequestPhasePending,
			liveDef:       true,
			accountExists: true,
			disabled:      true,
		},
		{
			name:          "claimed/live definition and component metadata",
			phase:         systemaccount.RestoreRequestPhaseClaimed,
			liveDef:       true,
			accountExists: true,
			expectTarget:  true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
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
			cachedComponent := &appsv1.Component{
				TypeMeta: metav1.TypeMeta{
					APIVersion: appsv1.GroupVersion.String(),
					Kind:       appsv1.ComponentKind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "cluster-mysql",
					UID:       "component-uid",
					Labels: constant.GetCompLabels(
						"cluster", "mysql"),
				},
				Spec: appsv1.ComponentSpec{
					CompDef:     "mysql",
					Labels:      map[string]string{"dynamic-source": "cached-component"},
					Annotations: map[string]string{"dynamic-source": "cached-component"},
				},
			}
			liveComponent := cachedComponent.DeepCopy()
			liveComponent.Spec.Labels["dynamic-source"] = "live-component"
			liveComponent.Spec.Annotations["dynamic-source"] = "live-component"
			if test.disabled {
				liveComponent.Spec.SystemAccounts = []appsv1.ComponentSystemAccount{{
					Name:     "admin",
					Disabled: ptr.To(true),
				}}
			}
			cachedDefinition := &appsv1.ComponentDefinition{
				ObjectMeta: metav1.ObjectMeta{Name: "mysql"},
				Spec: appsv1.ComponentDefinitionSpec{
					SystemAccounts: []appsv1.SystemAccount{{Name: "admin"}},
					Labels:         map[string]string{"static-source": "cached-definition"},
					Annotations:    map[string]string{"static-source": "cached-definition"},
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
				Namespace:  cachedComponent.Namespace,
				Name:       cachedComponent.Name,
				UID:        cachedComponent.UID,
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
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
				string(test.phase)

			objects := []client.Object{root, liveComponent, request}
			if test.liveDef {
				liveDefinition := cachedDefinition.DeepCopy()
				if !test.accountExists {
					liveDefinition.Spec.SystemAccounts = nil
				}
				liveDefinition.Spec.Labels["static-source"] = "live-definition"
				liveDefinition.Spec.Annotations["static-source"] = "live-definition"
				objects = append(objects, liveDefinition)
			}
			baseClient := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(objects...).Build()
			graphCli := model.NewGraphClient(baseClient)
			dag := graph.NewDAG()
			graphCli.Root(dag, cachedComponent.DeepCopy(), cachedComponent, model.ActionStatusPtr())
			transCtx := &componentTransformContext{
				Context:       context.Background(),
				Client:        graphCli,
				APIReader:     graphCli,
				CompDef:       cachedDefinition,
				Component:     cachedComponent,
				ComponentOrig: cachedComponent.DeepCopy(),
				SynthesizeComponent: &componentctrl.SynthesizedComponent{
					Namespace:          cachedComponent.Namespace,
					ClusterName:        root.Name,
					Name:               "mysql",
					StaticLabels:       map[string]string{"static-source": "cached-synthesized"},
					DynamicLabels:      map[string]string{"dynamic-source": "cached-synthesized"},
					StaticAnnotations:  map[string]string{"static-source": "cached-synthesized"},
					DynamicAnnotations: map[string]string{"dynamic-source": "cached-synthesized"},
				},
			}

			transformErr := (&componentAccountTransformer{}).Transform(transCtx, dag)

			require.True(t, intctrlutil.IsRequeueError(transformErr), transformErr)
			secrets := graphCli.FindAll(dag, &corev1.Secret{})
			if test.expectTarget {
				var target *corev1.Secret
				for _, object := range secrets {
					secret := object.(*corev1.Secret)
					if secret.Name == constant.GenerateAccountSecretName(
						root.Name, "mysql", "admin") {
						target = secret
					}
				}
				require.NotNil(t, target)
				require.Equal(t, "live-definition", target.Labels["static-source"])
				require.Equal(t, "live-component", target.Labels["dynamic-source"])
				require.Equal(t, "live-definition", target.Annotations["static-source"])
				require.Equal(t, "live-component", target.Annotations["dynamic-source"])
				return
			}
			var updatedRequest *corev1.Secret
			for _, object := range secrets {
				secret := object.(*corev1.Secret)
				if secret.Name == request.Name {
					updatedRequest = secret
				}
				require.NotEqual(t,
					constant.GenerateAccountSecretName(root.Name, "mysql", "admin"),
					secret.Name, "unavailable live account authority must prevent target mutation")
			}
			require.NotNil(t, updatedRequest)
			require.Equal(t, string(systemaccount.RestoreRequestPhaseFailed),
				updatedRequest.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
			require.Equal(t, systemaccount.AccountUnavailableReason,
				updatedRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
		})
	}
}
