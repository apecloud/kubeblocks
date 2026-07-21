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
	"errors"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/lifecycle"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
	kbacli "github.com/apecloud/kubeblocks/pkg/kbagent/client"
)

// TestJoinMember4ScaleOutFailureVisibility verifies that when the memberJoin
// lifecycle action fails during scale-out:
//  1. a Warning event is emitted on the Component so the failure is visible, and
//  2. a *delayed* requeue error is returned so that the component status
//     transformer still runs and the component phase/conditions stay live.
func TestJoinMember4ScaleOutFailureVisibility(t *testing.T) {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
		compName    = "mysql"
	)
	fullCompName := clusterName + "-" + compName
	podName := fullCompName + "-1"

	// the memberJoin lifecycle action fails with a kbagent error
	kbacli.SetMockClient(nil, errors.New("memberJoin failed: rc: 1, stderr: node is not reachable"))
	defer kbacli.UnsetMockClient()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
			Labels:    constant.GetCompLabels(clusterName, compName),
		},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()

	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fullCompName,
			Labels: map[string]string{
				constant.AppInstanceLabelKey:    clusterName,
				constant.KBAppComponentLabelKey: compName,
			},
		},
	}

	runningITS := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fullCompName},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To[int32](2)},
	}
	protoITS := runningITS.DeepCopy()
	// replicas-status: the new replica has been provisioned but has not joined yet
	require.NoError(t, component.NewReplicasStatus(protoITS, []string{podName}, true, false))

	synthesizedComp := &component.SynthesizedComponent{
		Namespace:   namespace,
		ClusterName: clusterName,
		Name:        compName,
		Replicas:    2,
		LifecycleActions: component.SynthesizedLifecycleActions{
			ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
				MemberJoin: &appsv1.Action{
					Exec: &appsv1.ExecAction{Command: []string{"/scripts/member-join.sh"}},
				},
			},
		},
	}

	recorder := record.NewFakeRecorder(8)
	transCtx := &componentTransformContext{
		Context:             context.Background(),
		Client:              cli,
		EventRecorder:       recorder,
		Logger:              logr.Discard(),
		Component:           comp,
		SynthesizeComponent: synthesizedComp,
		RunningWorkload:     runningITS,
		ProtoWorkload:       protoITS,
	}
	ops := &componentWorkloadOps{
		transCtx:       transCtx,
		cli:            cli,
		component:      comp,
		synthesizeComp: synthesizedComp,
		runningITS:     runningITS,
		protoITS:       protoITS,
	}

	err := ops.joinMember4ScaleOut()
	require.Error(t, err)
	require.True(t, intctrlutil.IsDelayedRequeueError(err),
		"member join failure should return a delayed requeue error so the status transformer still runs, got: %v", err)

	select {
	case event := <-recorder.Events:
		require.Contains(t, event, corev1.EventTypeWarning)
		require.Contains(t, event, "MemberJoinFailed")
		require.True(t, strings.Contains(event, "member join"), "event should describe the member join failure: %s", event)
	default:
		t.Fatalf("expected a Warning event to be emitted on the component for the member join failure")
	}
}

func TestJoinMember4ScaleOutPendingReplicaDoesNotReportFailure(t *testing.T) {
	const (
		namespace   = "default"
		clusterName = "test-cluster"
		compName    = "mysql"
	)
	fullCompName := clusterName + "-" + compName
	podName := fullCompName + "-1"

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	cli := fake.NewClientBuilder().WithScheme(scheme).Build()
	comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
		Namespace: namespace,
		Name:      fullCompName,
		Labels: map[string]string{
			constant.AppInstanceLabelKey:    clusterName,
			constant.KBAppComponentLabelKey: compName,
		},
	}}
	runningITS := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fullCompName},
		Spec:       workloads.InstanceSetSpec{Replicas: ptr.To[int32](2)},
	}
	protoITS := runningITS.DeepCopy()
	require.NoError(t, component.NewReplicasStatus(protoITS, []string{podName}, true, false))
	recorder := record.NewFakeRecorder(8)
	synthesizedComp := &component.SynthesizedComponent{
		Namespace: namespace, ClusterName: clusterName, Name: compName, Replicas: 2,
	}
	transCtx := &componentTransformContext{
		Context: context.Background(), Client: cli, EventRecorder: recorder, Logger: logr.Discard(),
		Component: comp, SynthesizeComponent: synthesizedComp,
		RunningWorkload: runningITS, ProtoWorkload: protoITS,
	}
	ops := &componentWorkloadOps{
		transCtx: transCtx, cli: cli, component: comp, synthesizeComp: synthesizedComp,
		runningITS: runningITS, protoITS: protoITS,
	}

	err := ops.joinMember4ScaleOut()
	require.Error(t, err)
	require.Truef(t, intctrlutil.IsDelayedRequeueError(err), "expected delayed requeue, got %T: %v", err, err)
	select {
	case event := <-recorder.Events:
		t.Fatalf("pending replica without a lifecycle invocation must not emit a failure event: %s", event)
	default:
	}
}

func TestJoinMember4ScaleOutRetryableActionDoesNotReportFailure(t *testing.T) {
	for name, actionErr := range map[string]error{
		"in progress": lifecycle.ErrActionInProgress,
		"busy":        lifecycle.ErrActionBusy,
	} {
		t.Run(name, func(t *testing.T) {
			const (
				namespace   = "default"
				clusterName = "test-cluster"
				compName    = "mysql"
			)
			fullCompName := clusterName + "-" + compName
			podName := fullCompName + "-1"

			kbacli.SetMockClient(nil, actionErr)
			defer kbacli.UnsetMockClient()

			scheme := runtime.NewScheme()
			require.NoError(t, clientgoscheme.AddToScheme(scheme))
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      podName,
				Labels:    constant.GetCompLabels(clusterName, compName),
			}}
			cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
			comp := &appsv1.Component{ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      fullCompName,
				Labels: map[string]string{
					constant.AppInstanceLabelKey:    clusterName,
					constant.KBAppComponentLabelKey: compName,
				},
			}}
			runningITS := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: fullCompName},
				Spec:       workloads.InstanceSetSpec{Replicas: ptr.To[int32](2)},
			}
			protoITS := runningITS.DeepCopy()
			require.NoError(t, component.NewReplicasStatus(protoITS, []string{podName}, true, false))
			recorder := record.NewFakeRecorder(8)
			synthesizedComp := &component.SynthesizedComponent{
				Namespace: namespace, ClusterName: clusterName, Name: compName, Replicas: 2,
				LifecycleActions: component.SynthesizedLifecycleActions{
					ComponentLifecycleActions: &appsv1.ComponentLifecycleActions{
						MemberJoin: &appsv1.Action{
							Exec: &appsv1.ExecAction{Command: []string{"/scripts/member-join.sh"}},
						},
					},
				},
			}
			transCtx := &componentTransformContext{
				Context: context.Background(), Client: cli, EventRecorder: recorder, Logger: logr.Discard(),
				Component: comp, SynthesizeComponent: synthesizedComp,
				RunningWorkload: runningITS, ProtoWorkload: protoITS,
			}
			ops := &componentWorkloadOps{
				transCtx: transCtx, cli: cli, component: comp, synthesizeComp: synthesizedComp,
				runningITS: runningITS, protoITS: protoITS,
			}

			err := ops.joinMember4ScaleOut()
			require.Error(t, err)
			require.Truef(t, intctrlutil.IsDelayedRequeueError(err), "expected delayed requeue, got %T: %v", err, err)
			select {
			case event := <-recorder.Events:
				t.Fatalf("retryable lifecycle state must not emit a failure event: %s", event)
			default:
			}
		})
	}
}
