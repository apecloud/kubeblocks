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

package instanceset

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

func TestHandleRoleChangedEventPlainExclusiveRole(t *testing.T) {
	tests := []struct {
		name                 string
		role                 string
		pods                 []*corev1.Pod
		targetPod            string
		wantRoles            map[string]string
		wantTargetAnnotation string
	}{
		{
			name:                 "bootstraps exclusive role when no active holder exists",
			role:                 "leader",
			pods:                 []*corev1.Pod{testRolePod("foo-0", "uid-0", "follower", nil)},
			targetPod:            "foo-0",
			wantRoles:            map[string]string{"foo-0": "leader"},
			wantTargetAnnotation: "200000000",
		},
		{
			name:                 "demotes the current exclusive holder through a plain non-exclusive role event",
			role:                 "follower",
			pods:                 []*corev1.Pod{testRolePod("foo-0", "uid-0", "leader", nil)},
			targetPod:            "foo-0",
			wantRoles:            map[string]string{"foo-0": "follower"},
			wantTargetAnnotation: "200000000",
		},
		{
			name: "does not promote a plain exclusive role event when another active holder exists",
			role: "leader",
			pods: []*corev1.Pod{
				testRolePod("foo-0", "uid-0", "leader", map[string]string{
					constant.LastRoleSnapshotVersionAnnotationKey: "100",
				}),
				testRolePod("foo-2", "uid-2", "follower", map[string]string{
					constant.LastRoleSnapshotVersionAnnotationKey: "100",
				}),
			},
			targetPod:            "foo-2",
			wantRoles:            map[string]string{"foo-0": "leader", "foo-2": "follower"},
			wantTargetAnnotation: "100",
		},
		{
			name: "does not use a plain exclusive role event to clean up another active holder",
			role: "leader",
			pods: []*corev1.Pod{
				testRolePod("foo-0", "uid-0", "leader", map[string]string{
					constant.LastRoleSnapshotVersionAnnotationKey: "100",
				}),
				testRolePod("foo-2", "uid-2", "leader", map[string]string{
					constant.LastRoleSnapshotVersionAnnotationKey: "100",
				}),
			},
			targetPod:            "foo-0",
			wantRoles:            map[string]string{"foo-0": "leader", "foo-2": "leader"},
			wantTargetAnnotation: "100",
		},
		{
			name: "allows plain exclusive promotion when the only other holder is terminating",
			role: "leader",
			pods: []*corev1.Pod{
				testRolePod("foo-0", "uid-0", "leader", nil, testRolePodDeleting()),
				testRolePod("foo-2", "uid-2", "follower", nil),
			},
			targetPod:            "foo-2",
			wantRoles:            map[string]string{"foo-0": "", "foo-2": "leader"},
			wantTargetAnnotation: "200000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := testRoleClient(t, testRoleInstanceSet(), tt.pods...)
			target := findTestRolePod(t, tt.pods, tt.targetPod)
			event := testRoleEvent(t, target, tt.role)

			if _, err := handleRoleChangedEvent(cli, testRoleRequestCtx(), nil, event); err != nil {
				t.Fatalf("handle role event: %v", err)
			}

			for podName, wantRole := range tt.wantRoles {
				got := &corev1.Pod{}
				if err := cli.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: podName}, got); err != nil {
					t.Fatalf("get pod %s: %v", podName, err)
				}
				if got.Labels[RoleLabelKey] != wantRole {
					t.Fatalf("pod %s role = %q, want %q", podName, got.Labels[RoleLabelKey], wantRole)
				}
			}

			gotTarget := &corev1.Pod{}
			if err := cli.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: tt.targetPod}, gotTarget); err != nil {
				t.Fatalf("get target pod %s: %v", tt.targetPod, err)
			}
			if gotTarget.Annotations[constant.LastRoleSnapshotVersionAnnotationKey] != tt.wantTargetAnnotation {
				t.Fatalf("target pod annotation = %q, want %q",
					gotTarget.Annotations[constant.LastRoleSnapshotVersionAnnotationKey], tt.wantTargetAnnotation)
			}
		})
	}
}

func testRoleClient(t *testing.T, its *workloads.InstanceSet, pods ...*corev1.Pod) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := workloads.AddToScheme(scheme); err != nil {
		t.Fatalf("add workloads scheme: %v", err)
	}

	objects := make([]client.Object, 0, len(pods)+1)
	objects = append(objects, its)
	for _, pod := range pods {
		objects = append(objects, pod)
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func testRoleInstanceSet() *workloads.InstanceSet {
	return &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: workloads.InstanceSetSpec{
			Roles: []workloads.ReplicaRole{
				{
					Name:                 "leader",
					ParticipatesInQuorum: true,
					UpdatePriority:       5,
					IsExclusive:          true,
				},
				{
					Name:                 "follower",
					ParticipatesInQuorum: true,
					UpdatePriority:       3,
				},
			},
		},
	}
}

func testRolePod(podName, podUID, role string, annotations map[string]string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        podName,
			UID:         types.UID(podUID),
			Labels:      testRolePodLabels(role),
			Annotations: annotations,
		},
	}
	for _, opt := range opts {
		opt(pod)
	}
	return pod
}

func testRolePodDeleting() func(*corev1.Pod) {
	return func(pod *corev1.Pod) {
		now := metav1.NewTime(time.Unix(100, 0))
		pod.DeletionTimestamp = &now
		pod.Finalizers = []string{"keep-terminating-state"}
	}
}

func testRolePodLabels(role string) map[string]string {
	return map[string]string{
		constant.AppManagedByLabelKey: constant.AppName,
		WorkloadsManagedByLabelKey:    workloads.InstanceSetKind,
		WorkloadsInstanceLabelKey:     name,
		RoleLabelKey:                  role,
	}
}

func findTestRolePod(t *testing.T, pods []*corev1.Pod, podName string) *corev1.Pod {
	t.Helper()
	for _, pod := range pods {
		if pod.Name == podName {
			return pod
		}
	}
	t.Fatalf("pod %s not found", podName)
	return nil
}

func testRoleEvent(t *testing.T, pod *corev1.Pod, role string) *corev1.Event {
	t.Helper()
	message, err := json.Marshal(probeMessage{
		Event: successEvent,
		Role:  role,
	})
	if err != nil {
		t.Fatalf("marshal probe message: %v", err)
	}
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      pod.Name + ".role",
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  namespace,
			Name:       pod.Name,
			UID:        pod.UID,
			FieldPath:  lorryEventFieldPath,
		},
		Reason:    checkRoleOperation,
		Message:   string(message),
		EventTime: metav1.NewMicroTime(time.Unix(200, 0)),
	}
}

func testRoleRequestCtx() intctrlutil.RequestCtx {
	return intctrlutil.RequestCtx{
		Ctx: context.Background(),
		Log: logf.FromContext(context.Background()),
	}
}
