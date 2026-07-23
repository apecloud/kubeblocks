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

package apps

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/systemaccount"
)

func TestSystemAccountRestoreLifecycleReleasesGoneRoot(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("protocol finalizer was not released: %#v", current.Finalizers)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RootUnavailableReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecyclePersistsDeletionFailureBeforeRelease(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	now := metav1.NewTime(time.Now())
	request.DeletionTimestamp = &now
	cluster := &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.Namespace,
			Name:      request.Annotations[systemaccount.RootClusterNameAnnotationKey],
			UID:       types.UID(request.Annotations[systemaccount.RootClusterUIDAnnotationKey]),
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup:  "dataprotection.kubeblocks.io",
					Kind:      "Backup",
					Namespace: request.Namespace,
					Name:      "backup",
				},
			},
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("protocol finalizer released before failure projection")
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RequestDeletionRequestedReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecyclePersistsDeletionFailureBeforeExactReceiptCleanup(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	now := metav1.NewTime(time.Now())
	request.DeletionTimestamp = &now
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	originalData := target.DeepCopy().Data
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	currentRequest := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", currentRequest.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.RequestDeletionRequestedReason {
		t.Fatalf("reason = %q", currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	currentTarget := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(currentTarget.Data, originalData) {
		t.Fatal("deleting request changed target credential data")
	}
	if currentTarget.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] == "" {
		t.Fatal("target receipt was cleared before deletion failure became durable")
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		systemaccount.RestoreOperationDigestAnnotationKey,
		systemaccount.CredentialIntentRevisionAnnotationKey,
		systemaccount.TargetCommitRevisionAnnotationKey,
		systemaccount.RestoreRequestNameAnnotationKey,
		systemaccount.RestoreRequestUIDAnnotationKey,
	} {
		if currentTarget.Annotations[key] != "" {
			t.Fatalf("target retained protocol receipt %s=%q", key, currentTarget.Annotations[key])
		}
	}
}

func TestSystemAccountRestoreLifecycleTerminalOperationPrecedesNewDeletionFailure(t *testing.T) {
	for _, rootState := range []string{
		"terminal-success",
		"terminal-failure",
		"gone",
		"terminating",
	} {
		for _, initialPhase := range []systemaccount.RestoreRequestPhase{
			systemaccount.RestoreRequestPhaseClaimed,
			systemaccount.RestoreRequestPhaseCommitted,
		} {
			t.Run(rootState+"/"+string(initialPhase), func(t *testing.T) {
				scheme := newSystemAccountLifecycleScheme(t)
				request := newLifecycleRestoreRequest(t)
				request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
					string(initialPhase)
				now := metav1.NewTime(time.Now())
				request.DeletionTimestamp = &now
				component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "component",
					UID:       "component-uid",
				}}
				target := newLifecycleExactTarget(t, scheme, request, component)
				objects := []client.Object{component, request, target}
				switch rootState {
				case "terminal-success", "terminal-failure":
					cluster := newLifecycleActiveCluster(request)
					status := metav1.ConditionTrue
					if rootState == "terminal-failure" {
						status = metav1.ConditionFalse
					}
					cluster.Status.Conditions = []metav1.Condition{{
						Type:               appsv1.ConditionTypeRestore,
						Status:             status,
						ObservedGeneration: cluster.Generation,
					}}
					objects = append(objects, cluster)
				case "terminating":
					cluster := newLifecycleActiveCluster(request)
					cluster.Finalizers = []string{"test.kubeblocks.io/hold"}
					cluster.DeletionTimestamp = &now
					objects = append(objects, cluster)
				}
				cli := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(objects...).Build()
				reconciler := &SystemAccountRestoreLifecycleReconciler{
					Client: cli, APIReader: cli, Scheme: scheme,
				}
				key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

				if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
					t.Fatal(err)
				}
				currentRequest := &corev1.Secret{}
				requestErr := cli.Get(context.Background(),
					client.ObjectKeyFromObject(request), currentRequest)
				if requestErr == nil &&
					currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] ==
						systemaccount.RequestDeletionRequestedReason {
					t.Fatal("terminal entry was rewritten as a manual-deletion failure")
				}
				if requestErr != nil && !apierrors.IsNotFound(requestErr) {
					t.Fatal(requestErr)
				}
				currentTarget := &corev1.Secret{}
				if err := cli.Get(context.Background(),
					client.ObjectKeyFromObject(target), currentTarget); err != nil {
					t.Fatal(err)
				}
				if !systemaccount.TargetReceiptExactV2(
					currentTarget, request, constant.DBComponentFinalizerName) {
					t.Fatal("terminal-first cleanup unexpectedly cleared the target receipt")
				}
			})
		}
	}
}

func TestSystemAccountRestoreLifecycleContinuesDurableDeletionFailureAfterOperationStops(t *testing.T) {
	for _, rootState := range []string{
		"terminal-success",
		"terminal-failure",
		"gone",
		"terminating",
	} {
		t.Run(rootState, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
				string(systemaccount.RestoreRequestPhaseFailed)
			request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] =
				systemaccount.RequestDeletionRequestedReason
			now := metav1.NewTime(time.Now())
			request.DeletionTimestamp = &now
			component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "component",
				UID:       "component-uid",
			}}
			target := newLifecycleExactTarget(t, scheme, request, component)
			objects := []client.Object{component, request, target}
			switch rootState {
			case "terminal-success", "terminal-failure":
				cluster := newLifecycleActiveCluster(request)
				status := metav1.ConditionTrue
				if rootState == "terminal-failure" {
					status = metav1.ConditionFalse
				}
				cluster.Status.Conditions = []metav1.Condition{{
					Type:               appsv1.ConditionTypeRestore,
					Status:             status,
					ObservedGeneration: cluster.Generation,
				}}
				objects = append(objects, cluster)
			case "terminating":
				cluster := newLifecycleActiveCluster(request)
				cluster.Finalizers = []string{"test.kubeblocks.io/hold"}
				cluster.DeletionTimestamp = &now
				objects = append(objects, cluster)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(objects...).Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(request),
			}); err != nil {
				t.Fatal(err)
			}
			currentTarget := &corev1.Secret{}
			if err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(target), currentTarget); err != nil {
				t.Fatal(err)
			}
			if currentTarget.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] != "" {
				t.Fatal("durable deletion failure retained exact target receipt")
			}
			currentRequest := &corev1.Secret{}
			err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(request), currentRequest)
			if err == nil && slices.Contains(currentRequest.Finalizers,
				systemaccount.RestoreProtocolFinalizer) {
				t.Fatal("durable deletion failure retained protocol finalizer")
			}
			if err != nil && !apierrors.IsNotFound(err) {
				t.Fatal(err)
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleRetainsFinalizerAcrossReceiptClearFailure(t *testing.T) {
	for _, mode := range []string{"write-failed", "response-lost"} {
		t.Run(mode, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
				string(systemaccount.RestoreRequestPhaseFailed)
			request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] =
				systemaccount.RequestDeletionRequestedReason
			now := metav1.NewTime(time.Now())
			request.DeletionTimestamp = &now
			cluster := newLifecycleActiveCluster(request)
			cluster.Status.Conditions = []metav1.Condition{{
				Type:               appsv1.ConditionTypeRestore,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: cluster.Generation,
			}}
			component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "component",
				UID:       "component-uid",
			}}
			target := newLifecycleExactTarget(t, scheme, request, component)
			originalData := target.DeepCopy().Data
			injected := false
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(cluster, component, request, target).
				WithInterceptorFuncs(interceptor.Funcs{
					Update: func(ctx context.Context, cli client.WithWatch, object client.Object,
						opts ...client.UpdateOption) error {
						secret, ok := object.(*corev1.Secret)
						if !injected && ok && secret.Name == target.Name &&
							secret.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] == "" {
							injected = true
							if mode == "response-lost" {
								if err := cli.Update(ctx, object, opts...); err != nil {
									return err
								}
							}
							return errors.New("injected target receipt clear failure")
						}
						return cli.Update(ctx, object, opts...)
					},
				}).
				Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}
			key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

			if _, err := reconciler.Reconcile(context.Background(), key); err == nil {
				t.Fatal("injected target receipt clear failure was not observed")
			}
			currentRequest := &corev1.Secret{}
			if err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(request), currentRequest); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(currentRequest.Finalizers,
				systemaccount.RestoreProtocolFinalizer) {
				t.Fatal("receipt clear error released protocol finalizer")
			}

			if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
				t.Fatal(err)
			}
			currentTarget := &corev1.Secret{}
			if err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(target), currentTarget); err != nil {
				t.Fatal(err)
			}
			if currentTarget.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] != "" {
				t.Fatal("retry retained exact target receipt")
			}
			if !reflect.DeepEqual(currentTarget.Data, originalData) {
				t.Fatal("receipt cleanup changed credential data")
			}
			err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(request), currentRequest)
			if err == nil && slices.Contains(currentRequest.Finalizers,
				systemaccount.RestoreProtocolFinalizer) {
				t.Fatal("successful retry retained protocol finalizer")
			}
			if err != nil && !apierrors.IsNotFound(err) {
				t.Fatal(err)
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleDeletingStableFailureRetriesReceiptCleanup(t *testing.T) {
	for _, reason := range []string{
		systemaccount.TargetSemanticUnavailableReason,
		systemaccount.RequestDeletionRequestedReason,
		systemaccount.PostWriteCancellationReason,
	} {
		t.Run(reason, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
				string(systemaccount.RestoreRequestPhaseFailed)
			request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] = reason
			now := metav1.NewTime(time.Now())
			request.DeletionTimestamp = &now
			cluster := newLifecycleActiveCluster(request)
			component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "component",
				UID:       "component-uid",
			}}
			target := newLifecycleExactTarget(t, scheme, request, component)
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(cluster, component, request, target).Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(request),
			}); err != nil {
				t.Fatal(err)
			}
			currentTarget := &corev1.Secret{}
			if err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(target), currentTarget); err != nil {
				t.Fatal(err)
			}
			if currentTarget.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] != "" {
				t.Fatal("deleting stable failure retained exact target receipt")
			}
			currentRequest := &corev1.Secret{}
			if err := cli.Get(context.Background(),
				client.ObjectKeyFromObject(request), currentRequest); err != nil {
				t.Fatal(err)
			}
			if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] != reason {
				t.Fatalf("reason = %q",
					currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
			}
			if !slices.Contains(currentRequest.Finalizers,
				systemaccount.RestoreProtocolFinalizer) {
				t.Fatal("active operation released protocol finalizer")
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleReleasesDamagedMetadataAfterRootGone(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.OwnerReferences[0].UID = "replacement-root-uid"
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatalf("damaged metadata kept protocol finalizer after root disappeared")
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.InvalidPhaseReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
}

func TestSystemAccountRestoreLifecycleRecoversDamagedProtocolMirror(t *testing.T) {
	for _, protocol := range []string{"", "foreign"} {
		t.Run(protocol, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			if protocol == "" {
				delete(request.Annotations, systemaccount.RestoreProtocolAnnotationKey)
			} else {
				request.Annotations[systemaccount.RestoreProtocolAnnotationKey] = protocol
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(request).Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(request),
			}); err != nil {
				t.Fatal(err)
			}

			current := &corev1.Secret{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
				t.Fatal(err)
			}
			if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				t.Fatalf("protocol %q kept finalizer after root disappeared", protocol)
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleFailsClosedForDamagedMetadataWhileRootLive(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.OwnerReferences[0].UID = "replacement-root-uid"
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}, Spec: appsv1.ClusterSpec{
		Restore: &appsv1.ClusterRestore{
			Source: appsv1.ClusterRestoreSource{
				APIGroup:  "dataprotection.kubeblocks.io",
				Kind:      "Backup",
				Namespace: "default",
				Name:      "backup",
			},
		},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err == nil {
		t.Fatal("active root accepted damaged request metadata")
	}
}

func TestSystemAccountRestoreLifecycleDamagedProtocolMirrorFailsClosedWhileRootLive(t *testing.T) {
	for _, protocol := range []string{"", "foreign"} {
		t.Run(protocol, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			if protocol == "" {
				delete(request.Annotations, systemaccount.RestoreProtocolAnnotationKey)
			} else {
				request.Annotations[systemaccount.RestoreProtocolAnnotationKey] = protocol
			}
			cluster := newLifecycleActiveCluster(request)
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(cluster, request).Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(request),
			}); err == nil {
				t.Fatalf("active root accepted protocol mirror %q", protocol)
			}
			current := &corev1.Secret{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				t.Fatalf("active root released finalizer for protocol mirror %q", protocol)
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleDamagedProtocolMirrorReleasesAfterOperationTerminal(t *testing.T) {
	for _, status := range []metav1.ConditionStatus{
		metav1.ConditionTrue,
		metav1.ConditionFalse,
	} {
		for _, protocol := range []string{"", "foreign"} {
			t.Run(string(status)+"/"+protocol, func(t *testing.T) {
				scheme := newSystemAccountLifecycleScheme(t)
				request := newLifecycleRestoreRequest(t)
				if protocol == "" {
					delete(request.Annotations, systemaccount.RestoreProtocolAnnotationKey)
				} else {
					request.Annotations[systemaccount.RestoreProtocolAnnotationKey] = protocol
				}
				cluster := newLifecycleActiveCluster(request)
				cluster.Status.Conditions = []metav1.Condition{{
					Type:               appsv1.ConditionTypeRestore,
					Status:             status,
					ObservedGeneration: cluster.Generation,
				}}
				cli := fake.NewClientBuilder().WithScheme(scheme).
					WithObjects(cluster, request).Build()
				reconciler := &SystemAccountRestoreLifecycleReconciler{
					Client: cli, APIReader: cli, Scheme: scheme,
				}

				if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
					NamespacedName: client.ObjectKeyFromObject(request),
				}); err != nil {
					t.Fatal(err)
				}
				current := &corev1.Secret{}
				if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
					t.Fatal(err)
				}
				if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
					t.Fatalf("terminal operation retained finalizer for protocol mirror %q", protocol)
				}
				if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
					systemaccount.InvalidPhaseReason {
					t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
				}
			})
		}
	}
}

func TestSystemAccountRestoreLifecyclePersistsPostWriteCancellationBeforeReceiptClear(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}
	key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

	if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	currentRequest := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.PostWriteCancellationReason {
		t.Fatalf("reason = %q", currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	currentTarget := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if !systemaccount.TargetReceiptExactV2(
		currentTarget, currentRequest, constant.DBComponentFinalizerName) {
		t.Fatal("first pass cleared target receipt before durable cancellation")
	}
	if !slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("first pass released protocol finalizer before receipt clear")
	}

	if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.PostWriteCancellationReason {
		t.Fatalf("second-pass reason = %q",
			currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	if slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("gone-root second pass retained finalizer after receipt clear")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if systemaccount.TargetReceiptExactV2(
		currentTarget, currentRequest, constant.DBComponentFinalizerName) {
		t.Fatal("second pass retained exact target receipt")
	}
}

func TestSystemAccountRestoreLifecyclePersistsPostWriteCancellationForUnavailableOwner(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	originalData := target.DeepCopy().Data
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}
	key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

	if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	currentRequest := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.PostWriteCancellationReason {
		t.Fatalf("reason = %q", currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	currentTarget := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if !systemaccount.TargetReceiptExactV2(
		currentTarget, currentRequest, constant.DBComponentFinalizerName) {
		t.Fatal("first pass cleared target receipt before durable cancellation")
	}

	result, err := reconciler.Reconcile(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("active root with unavailable owner lost its bounded lifecycle wake")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("active root released request finalizer after target receipt clear")
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
		t.Fatal(err)
	}
	if systemaccount.TargetReceiptExactV2(
		currentTarget, currentRequest, constant.DBComponentFinalizerName) {
		t.Fatal("second pass retained exact target receipt")
	}
	if !reflect.DeepEqual(currentTarget.Data, originalData) {
		t.Fatal("post-write cancellation changed target credential data")
	}
}

func TestSystemAccountRestoreLifecycleRetainsPostWriteCancellationAcrossReleaseFailure(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	failRelease := false
	base := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(component, request, target)
	cli := base.WithInterceptorFuncs(interceptor.Funcs{
		Update: func(ctx context.Context, cli client.WithWatch, object client.Object,
			opts ...client.UpdateOption) error {
			secret, ok := object.(*corev1.Secret)
			if failRelease && ok && secret.Name == request.Name &&
				!slices.Contains(secret.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				failRelease = false
				return errors.New("injected request release failure")
			}
			return cli.Update(ctx, object, opts...)
		},
	}).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}
	key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

	if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	failRelease = true
	if _, err := reconciler.Reconcile(context.Background(), key); err == nil {
		t.Fatal("injected request release failure was not observed")
	}
	currentRequest := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.PostWriteCancellationReason {
		t.Fatalf("reason after release failure = %q",
			currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	if !slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("failed release unexpectedly removed finalizer")
	}

	if _, err := reconciler.Reconcile(context.Background(), key); err != nil {
		t.Fatal(err)
	}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
		t.Fatal(err)
	}
	if currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.PostWriteCancellationReason {
		t.Fatalf("recovered reason = %q",
			currentRequest.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	if slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("recovered release retained finalizer")
	}
}

func TestSystemAccountRestoreLifecycleCommitsExactTargetAfterActiveRecheck(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	intent, err := systemaccount.DecodeRestoreRequestV2(request)
	if err != nil {
		t.Fatal(err)
	}
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  request.Namespace,
			Name:       "cluster-component-admin",
			UID:        "target-uid",
			Finalizers: []string{constant.DBComponentFinalizerName},
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey:      "true",
				systemaccount.RestoreProtocolAnnotationKey:          systemaccount.RestoreProtocolV2,
				systemaccount.RestoreOperationDigestAnnotationKey:   request.Annotations[systemaccount.RestoreOperationDigestAnnotationKey],
				systemaccount.CredentialIntentRevisionAnnotationKey: request.Annotations[systemaccount.CredentialIntentRevisionAnnotationKey],
				systemaccount.RestoreRequestNameAnnotationKey:       request.Name,
				systemaccount.RestoreRequestUIDAnnotationKey:        string(request.UID),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: intent.Credentials,
	}
	if err := controllerutil.SetControllerReference(component, target, scheme); err != nil {
		t.Fatal(err)
	}
	revision, err := systemaccount.TargetCommitRevision(target, constant.DBComponentFinalizerName)
	if err != nil {
		t.Fatal(err)
	}
	target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = revision
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseCommitted) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.TargetSecretNameAnnotationKey] != target.Name ||
		current.Annotations[systemaccount.TargetSecretUIDAnnotationKey] != string(target.UID) ||
		current.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] != revision {
		t.Fatalf("committed receipt is incomplete: %#v", current.Annotations)
	}
	if !slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("active operation commit released the protocol finalizer")
	}
}

func TestSystemAccountRestoreLifecycleRepairsCommittedReceiptFromExactTarget(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseCommitted)
	request.Annotations[systemaccount.TargetSecretNameAnnotationKey] = "stale-target"
	request.Annotations[systemaccount.TargetSecretUIDAnnotationKey] = "stale-uid"
	request.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = "stale-revision"
	cluster := newLifecycleActiveCluster(request)
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	expectedRevision := target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey]
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseCommitted) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.TargetSecretNameAnnotationKey] != target.Name ||
		current.Annotations[systemaccount.TargetSecretUIDAnnotationKey] != string(target.UID) ||
		current.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] != expectedRevision {
		t.Fatalf("committed receipt was not repaired: %#v", current.Annotations)
	}
}

func TestSystemAccountRestoreLifecycleInvalidPhaseDoesNotHotWrite(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] = "Bogus"
	cluster := newLifecycleActiveCluster(request)
	updateCount := 0
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, request).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, cli client.WithWatch, obj client.Object,
				opts ...client.UpdateOption) error {
				updateCount++
				return cli.Update(ctx, obj, opts...)
			},
		}).
		Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}
	key := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(request)}

	for i := 0; i < 2; i++ {
		result, err := reconciler.Reconcile(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		if result.RequeueAfter == 0 {
			t.Fatal("invalid phase with a live root lost its bounded wake")
		}
	}
	if updateCount != 1 {
		t.Fatalf("invalid phase update count = %d, want 1", updateCount)
	}
}

func TestSystemAccountRestoreLifecycleDeletesCommittedRequestAfterFinalizerRelease(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseCommitted)
	request.Annotations[systemaccount.TargetSecretNameAnnotationKey] = "target"
	request.Annotations[systemaccount.TargetSecretUIDAnnotationKey] = "target-uid"
	request.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = "revision"
	request.Finalizers = nil
	updateCount := 0
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(request).
		WithInterceptorFuncs(interceptor.Funcs{
			Update: func(ctx context.Context, cli client.WithWatch, obj client.Object,
				opts ...client.UpdateOption) error {
				updateCount++
				return cli.Update(ctx, obj, opts...)
			},
		}).
		Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}
	if updateCount != 0 {
		t.Fatalf("released committed request update count = %d, want 0", updateCount)
	}
	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err == nil {
		t.Fatal("released committed request was not deleted after root disappeared")
	}
}

func TestSystemAccountRestoreLifecycleDeletesTerminalCommittedRequestWithExactTarget(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseCommitted)
	request.Finalizers = nil
	cluster := newLifecycleActiveCluster(request)
	cluster.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
	}}
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
	}}
	target := newLifecycleExactTarget(t, scheme, request, component)
	request.Annotations[systemaccount.TargetSecretNameAnnotationKey] = target.Name
	request.Annotations[systemaccount.TargetSecretUIDAnnotationKey] = string(target.UID)
	request.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] =
		target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey]
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, request, target).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{
		Client: cli, APIReader: cli, Scheme: scheme,
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err == nil {
		t.Fatal("terminal committed request with an exact target was not deleted")
	}
}

func TestSystemAccountRestoreLifecycleDoesNotCommitTerminalTargetWithUnavailableOwner(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	request := newLifecycleRestoreRequest(t)
	request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
		string(systemaccount.RestoreRequestPhaseClaimed)
	cluster := newLifecycleActiveCluster(request)
	cluster.Status.Conditions = []metav1.Condition{{
		Type:               appsv1.ConditionTypeRestore,
		Status:             metav1.ConditionTrue,
		ObservedGeneration: cluster.Generation,
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, request).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(request),
	}); err != nil {
		t.Fatal(err)
	}

	current := &corev1.Secret{}
	if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), current); err != nil {
		t.Fatal(err)
	}
	if current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] !=
		string(systemaccount.RestoreRequestPhaseFailed) {
		t.Fatalf("phase = %q", current.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey])
	}
	if current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] !=
		systemaccount.OperationTerminalReason {
		t.Fatalf("reason = %q", current.Annotations[systemaccount.RestoreRequestReasonAnnotationKey])
	}
	if slices.Contains(current.Finalizers, systemaccount.RestoreProtocolFinalizer) {
		t.Fatal("terminal unavailable-owner request retained the protocol finalizer")
	}
}

func TestSystemAccountRestoreLifecycleClearsStableFailureReceiptBeforeRelease(t *testing.T) {
	for _, rootState := range []string{"terminal", "gone"} {
		t.Run(rootState, func(t *testing.T) {
			scheme := newSystemAccountLifecycleScheme(t)
			request := newLifecycleRestoreRequest(t)
			request.Annotations[systemaccount.RestoreRequestPhaseAnnotationKey] =
				string(systemaccount.RestoreRequestPhaseFailed)
			request.Annotations[systemaccount.RestoreRequestReasonAnnotationKey] =
				systemaccount.TargetSemanticUnavailableReason
			component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
				Namespace: "default",
				Name:      "component",
				UID:       "component-uid",
			}}
			target := newLifecycleExactTarget(t, scheme, request, component)
			objects := []client.Object{component, request, target}
			if rootState == "terminal" {
				cluster := newLifecycleActiveCluster(request)
				cluster.Status.Conditions = []metav1.Condition{{
					Type:               appsv1.ConditionTypeRestore,
					Status:             metav1.ConditionTrue,
					ObservedGeneration: cluster.Generation,
				}}
				objects = append(objects, cluster)
			}
			cli := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(objects...).Build()
			reconciler := &SystemAccountRestoreLifecycleReconciler{
				Client: cli, APIReader: cli, Scheme: scheme,
			}

			if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(request),
			}); err != nil {
				t.Fatal(err)
			}

			currentTarget := &corev1.Secret{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(target), currentTarget); err != nil {
				t.Fatal(err)
			}
			if currentTarget.Annotations[systemaccount.RestoreRequestUIDAnnotationKey] != "" {
				t.Fatal("stable failed request released before its exact target receipt was cleared")
			}
			currentRequest := &corev1.Secret{}
			if err := cli.Get(context.Background(), client.ObjectKeyFromObject(request), currentRequest); err != nil {
				t.Fatal(err)
			}
			if slices.Contains(currentRequest.Finalizers, systemaccount.RestoreProtocolFinalizer) {
				t.Fatal("stable failed request retained protocol finalizer after exact receipt cleanup")
			}
		})
	}
}

func TestSystemAccountRestoreLifecycleRequiresExactAppsAuthorityGroup(t *testing.T) {
	scheme := newSystemAccountLifecycleScheme(t)
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	controller := true
	cluster := &appsv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "cluster",
		UID:       "cluster-uid",
	}}
	component := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "component",
		UID:       "component-uid",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.GroupVersion.String(),
			Kind:       appsv1.ClusterKind,
			Name:       cluster.Name,
			UID:        cluster.UID,
			Controller: &controller,
		}},
	}}
	workload := &workloads.InstanceSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "workload",
		UID:       "workload-uid",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: "apps.kubeblocks.io/v1alpha1",
			Kind:       appsv1.ComponentKind,
			Name:       component.Name,
			UID:        component.UID,
			Controller: &controller,
		}},
	}}
	pvc := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Namespace: "default",
		Name:      "data",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: workloads.GroupVersion.String(),
			Kind:       workloads.InstanceSetKind,
			Name:       workload.Name,
			UID:        workload.UID,
			Controller: &controller,
		}},
	}}
	cli := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(cluster, component, workload, pvc).Build()
	reconciler := &SystemAccountRestoreLifecycleReconciler{Client: cli, APIReader: cli, Scheme: scheme}

	live, err := reconciler.targetOwnerLive(context.Background(), systemaccount.ObjectIdentity{
		APIVersion: "apps.kubeblocks.io/v1alpha1",
		Kind:       appsv1.ComponentKind,
		Namespace:  component.Namespace,
		Name:       component.Name,
		UID:        component.UID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Fatal("target owner with a non-exact API version was accepted")
	}

	workload.OwnerReferences[0].APIVersion = appsv1.GroupVersion.String()
	if err := cli.Update(context.Background(), workload); err != nil {
		t.Fatal(err)
	}
	component.OwnerReferences[0].APIVersion = "apps.kubeblocks.io/v1alpha1"
	if err := cli.Update(context.Background(), component); err != nil {
		t.Fatal(err)
	}
}

func newSystemAccountLifecycleScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return scheme
}

func newLifecycleRestoreRequest(t *testing.T) *corev1.Secret {
	t.Helper()
	root := systemaccount.ObjectIdentity{
		APIVersion: appsv1.GroupVersion.String(),
		Kind:       appsv1.ClusterKind,
		Namespace:  "default",
		Name:       "cluster",
		UID:        "cluster-uid",
	}
	request, err := systemaccount.BuildRestoreRequest(systemaccount.CredentialIntent{
		Operation: systemaccount.RestoreOperationIdentity{
			Protocol: systemaccount.RestoreProtocolV2,
			Profile:  systemaccount.RestoreProfileInitialCluster,
			Root:     root,
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
			Root:      root,
			Owner: systemaccount.ObjectIdentity{
				APIVersion: appsv1.GroupVersion.String(),
				Kind:       appsv1.ComponentKind,
				Namespace:  "default",
				Name:       "component",
				UID:        "component-uid",
			},
			Scope:   systemaccount.SystemAccountScopeComponent,
			Account: "admin",
		},
		ResolvedSource: systemaccount.ObjectIdentity{
			APIVersion: "dataprotection.kubeblocks.io/v1alpha1",
			Kind:       "Backup",
			Namespace:  "default",
			Name:       "backup",
			UID:        "backup-uid",
		},
		Credentials: map[string][]byte{
			constant.AccountNameForSecret:   []byte("admin"),
			constant.AccountPasswdForSecret: []byte("password"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	request.UID = "request-uid"
	request.ResourceVersion = "1"
	return request
}

func newLifecycleActiveCluster(request *corev1.Secret) *appsv1.Cluster {
	return &appsv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: request.Namespace,
			Name:      request.Annotations[systemaccount.RootClusterNameAnnotationKey],
			UID:       types.UID(request.Annotations[systemaccount.RootClusterUIDAnnotationKey]),
		},
		Spec: appsv1.ClusterSpec{
			Restore: &appsv1.ClusterRestore{
				Source: appsv1.ClusterRestoreSource{
					APIGroup:  "dataprotection.kubeblocks.io",
					Kind:      "Backup",
					Namespace: request.Namespace,
					Name:      "backup",
				},
			},
		},
	}
}

func newLifecycleExactTarget(
	t *testing.T,
	scheme *runtime.Scheme,
	request *corev1.Secret,
	component *appsv1.Component,
) *corev1.Secret {
	t.Helper()
	intent, err := systemaccount.DecodeRestoreRequestV2(request)
	if err != nil {
		t.Fatal(err)
	}
	target := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  request.Namespace,
			Name:       "cluster-component-admin",
			UID:        "target-uid",
			Finalizers: []string{constant.DBComponentFinalizerName},
			Annotations: map[string]string{
				constant.SystemAccountProvisionedAnnotationKey:      "true",
				systemaccount.RestoreProtocolAnnotationKey:          systemaccount.RestoreProtocolV2,
				systemaccount.RestoreOperationDigestAnnotationKey:   request.Annotations[systemaccount.RestoreOperationDigestAnnotationKey],
				systemaccount.CredentialIntentRevisionAnnotationKey: request.Annotations[systemaccount.CredentialIntentRevisionAnnotationKey],
				systemaccount.RestoreRequestNameAnnotationKey:       request.Name,
				systemaccount.RestoreRequestUIDAnnotationKey:        string(request.UID),
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: intent.Credentials,
	}
	if err := controllerutil.SetControllerReference(component, target, scheme); err != nil {
		t.Fatal(err)
	}
	revision, err := systemaccount.TargetCommitRevision(target, constant.DBComponentFinalizerName)
	if err != nil {
		t.Fatal(err)
	}
	target.Annotations[systemaccount.TargetCommitRevisionAnnotationKey] = revision
	return target
}
