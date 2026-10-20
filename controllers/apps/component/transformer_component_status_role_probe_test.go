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
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestRoleProbeTimeoutRequiresUpdatedReadyWorkload(t *testing.T) {
	for _, tc := range []struct {
		name        string
		mutate      func(*workloads.InstanceSet)
		wantFailure bool
	}{
		{name: "stopped workload retains old ready condition", mutate: func(its *workloads.InstanceSet) {
			its.Status.Replicas = 0
			its.Status.ReadyReplicas = 0
			its.Status.UpdatedReplicas = 0
		}},
		{name: "workload has not observed start", mutate: func(its *workloads.InstanceSet) {
			its.Status.ObservedGeneration--
		}},
		{name: "workload belongs to previous component generation", mutate: func(its *workloads.InstanceSet) {
			its.Annotations[constant.KubeBlocksGenerationKey] = "1"
		}},
		{name: "replicas have not finished updating", mutate: func(its *workloads.InstanceSet) {
			its.Status.UpdatedReplicas = 1
		}},
		{name: "ready condition is stale while pods are not ready", mutate: func(its *workloads.InstanceSet) {
			its.Status.ReadyReplicas = 1
		}},
		{name: "ready condition is false", mutate: func(its *workloads.InstanceSet) {
			its.Status.Conditions[0].Status = metav1.ConditionFalse
		}},
		{name: "ready pods still have time to report roles", mutate: func(its *workloads.InstanceSet) {
			its.Status.Conditions[0].LastTransitionTime = metav1.Now()
		}},
		{name: "ready pods without roles time out", wantFailure: true},
		{name: "role probe completed", mutate: func(its *workloads.InstanceSet) {
			its.Status.InstanceStatus = []workloads.InstanceStatus{
				{PodName: "pod-0", Role: "primary"},
				{PodName: "pod-1", Role: "secondary"},
				{PodName: "pod-2", Role: "secondary"},
			}
		}},
		{name: "real instance failure is not hidden by pending update", mutate: func(its *workloads.InstanceSet) {
			its.Status.Replicas = 0
			its.Status.Conditions = append(its.Status.Conditions, metav1.Condition{
				Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue,
				Reason: "Failed", Message: "instance creation failed",
			})
		}, wantFailure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			its := &workloads.InstanceSet{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: 3,
					Annotations: map[string]string{constant.KubeBlocksGenerationKey: "2"}},
				Spec: workloads.InstanceSetSpec{Replicas: ptr.To(int32(3)), Roles: []workloads.ReplicaRole{{Name: "primary"}, {Name: "secondary"}}},
				Status: workloads.InstanceSetStatus{
					ObservedGeneration: 3, Replicas: 3, UpdatedReplicas: 3, ReadyReplicas: 3,
					Conditions: []metav1.Condition{{Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue,
						LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)), Reason: "Ready"}},
				},
			}
			if tc.mutate != nil {
				tc.mutate(its)
			}
			transformer := &componentStatusTransformer{
				comp: &appsv1.Component{ObjectMeta: metav1.ObjectMeta{Generation: 2}}, runningITS: its,
			}
			failed, messages := transformer.hasFailedPod()
			if failed != tc.wantFailure {
				t.Fatalf("hasFailedPod() = %v, want %v; messages: %v", failed, tc.wantFailure, messages)
			}
			if tc.wantFailure && len(messages) == 0 {
				t.Fatal("failure must include a diagnostic message")
			}
		})
	}
}
