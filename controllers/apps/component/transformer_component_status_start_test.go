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
	"strconv"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
)

func roleProbeStartFixture() (*componentStatusTransformer, *componentTransformContext) {
	comp := &appsv1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-orchestrator", Generation: 3},
		Spec:       appsv1.ComponentSpec{Replicas: 3},
		Status:     appsv1.ComponentStatus{Phase: appsv1.StoppedComponentPhase},
	}
	its := &workloads.InstanceSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cluster-orchestrator", Generation: 3,
			Annotations: map[string]string{constant.KubeBlocksGenerationKey: strconv.FormatInt(comp.Generation, 10)},
		},
		Spec: workloads.InstanceSetSpec{
			Replicas: ptr.To(int32(3)),
			Roles:    []workloads.ReplicaRole{{Name: "primary"}, {Name: "secondary"}},
		},
		Status: workloads.InstanceSetStatus{
			ObservedGeneration: 3, Replicas: 3, UpdatedReplicas: 3, ReadyReplicas: 3,
			Conditions: []metav1.Condition{{
				Type: string(workloads.InstanceReady), Status: metav1.ConditionTrue,
				ObservedGeneration: 3, LastTransitionTime: metav1.NewTime(time.Now().Add(-2 * time.Minute)),
			}},
		},
	}
	synth := &component.SynthesizedComponent{Replicas: 3}
	ctx := &componentTransformContext{
		Component: comp, ComponentOrig: comp.DeepCopy(),
		EventRecorder: record.NewFakeRecorder(100),
		CompDef:       &appsv1.ComponentDefinition{}, SynthesizeComponent: synth,
		RunningWorkload: its, ProtoWorkload: its.DeepCopy(),
	}
	return &componentStatusTransformer{
		comp: comp, synthesizeComp: synth, runningITS: its, protoITS: ctx.ProtoWorkload,
	}, ctx
}

func TestComponentRoleProbeTimeoutWaitsForReadyWorkload(t *testing.T) {
	tests := []struct {
		name   string
		change func(*componentStatusTransformer)
		failed bool
	}{
		{"current ready workload without roles still times out", func(*componentStatusTransformer) {}, true},
		{"component generation not applied", func(s *componentStatusTransformer) {
			s.runningITS.Annotations[constant.KubeBlocksGenerationKey] = "2"
		}, false},
		{"workload generation not observed", func(s *componentStatusTransformer) {
			s.runningITS.Status.ObservedGeneration = 2
		}, false},
		{"observed generation but zero pods after stop", func(s *componentStatusTransformer) {
			s.runningITS.Status.Replicas = 0
			s.runningITS.Status.UpdatedReplicas = 0
			s.runningITS.Status.ReadyReplicas = 0
		}, false},
		{"only some desired pods present", func(s *componentStatusTransformer) {
			s.runningITS.Status.Replicas = 1
			s.runningITS.Status.UpdatedReplicas = 1
			s.runningITS.Status.ReadyReplicas = 1
		}, false},
		{"not all pods updated", func(s *componentStatusTransformer) {
			s.runningITS.Status.UpdatedReplicas = 2
		}, false},
		{"not all pods ready despite stale true condition", func(s *componentStatusTransformer) {
			s.runningITS.Status.ReadyReplicas = 2
		}, false},
		{"ready condition belongs to previous generation", func(s *componentStatusTransformer) {
			s.runningITS.Status.Conditions[0].ObservedGeneration = 2
		}, false},
		{"pods not ready", func(s *componentStatusTransformer) {
			s.runningITS.Status.Conditions[0].Status = metav1.ConditionFalse
		}, false},
		{"pods only just became ready", func(s *componentStatusTransformer) {
			s.runningITS.Status.Conditions[0].LastTransitionTime = metav1.Now()
		}, false},
		{"roles already complete", func(s *componentStatusTransformer) {
			for _, name := range []string{"pod-0", "pod-1", "pod-2"} {
				s.runningITS.Status.InstanceStatus = append(s.runningITS.Status.InstanceStatus, workloads.InstanceStatus{
					PodName: name, Role: "secondary",
					DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
				})
			}
		}, false},
		{"explicit instance failure remains visible while updating", func(s *componentStatusTransformer) {
			s.runningITS.Status.ObservedGeneration = 2
			s.runningITS.Status.Conditions = append(s.runningITS.Status.Conditions, metav1.Condition{
				Type: string(workloads.InstanceFailure), Status: metav1.ConditionTrue, Message: "instance failed",
			})
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := roleProbeStartFixture()
			tt.change(s)
			failed, messages := s.hasFailedPod()
			if failed != tt.failed {
				t.Fatalf("hasFailedPod() = %v, want %v; messages=%v", failed, tt.failed, messages)
			}
		})
	}
}

func TestStartWithStaleReadyConditionRemainsStarting(t *testing.T) {
	s, ctx := roleProbeStartFixture()
	its := s.runningITS
	// Replay the status progression captured by #10987's pre-Start watches.
	// The old Ready=True condition is retained even after generation is observed.
	its.Status.ObservedGeneration = 2
	its.Status.Replicas, its.Status.UpdatedReplicas, its.Status.ReadyReplicas = 0, 0, 0
	its.Status.Conditions[0].ObservedGeneration = 2
	for _, observed := range []int64{2, 3} {
		its.Status.ObservedGeneration = observed
		its.Status.Conditions[0].ObservedGeneration = observed
		if err := s.reconcileStatus(ctx); err != nil {
			t.Fatal(err)
		}
		if s.comp.Status.Phase != appsv1.StartingComponentPhase {
			t.Fatalf("zero-pod phase at observedGeneration=%d = %s, want Starting; messages=%v",
				observed, s.comp.Status.Phase, s.comp.Status.Message)
		}
	}
	its.Status.Replicas, its.Status.UpdatedReplicas = 3, 3
	its.Status.Conditions[0].Status = metav1.ConditionFalse
	its.Status.Conditions[0].LastTransitionTime = metav1.Now()
	if err := s.reconcileStatus(ctx); err != nil {
		t.Fatal(err)
	}
	if s.comp.Status.Phase != appsv1.StartingComponentPhase {
		t.Fatalf("initializing pods phase = %s, want Starting", s.comp.Status.Phase)
	}
	its.Status.ReadyReplicas = 3
	its.Status.Conditions[0].Status = metav1.ConditionTrue
	its.Status.Conditions[0].LastTransitionTime = metav1.Now()
	for _, name := range []string{"pod-0", "pod-1", "pod-2"} {
		its.Status.InstanceStatus = append(its.Status.InstanceStatus, workloads.InstanceStatus{
			PodName: name, Role: "secondary",
			DesiredState: workloads.InstanceDesiredStateActive, CurrentState: workloads.InstanceCurrentStatePresent,
		})
	}
	if err := s.reconcileStatus(ctx); err != nil {
		t.Fatal(err)
	}
	if s.comp.Status.Phase != appsv1.RunningComponentPhase {
		t.Fatalf("ready pods and roles phase = %s, want Running", s.comp.Status.Phase)
	}
}
