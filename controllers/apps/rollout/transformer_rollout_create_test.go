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

package rollout

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestCreateReplicasRejectsTargetGreaterThanOriginal(t *testing.T) {
	transformer := &rolloutCreateTransformer{}
	rollout := &appsv1alpha1.Rollout{
		Status: appsv1alpha1.RolloutStatus{
			Components: []appsv1alpha1.RolloutComponentStatus{
				{
					Name:     "comp",
					Replicas: 3,
				},
			},
		},
	}
	comp := appsv1alpha1.RolloutComponent{
		Name:     "comp",
		Replicas: ptr.To(intstr.FromInt32(4)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{},
		},
	}
	spec := &appsv1.ClusterComponentSpec{Replicas: 3}

	_, _, err := transformer.replicas(rollout, comp, spec)
	if err == nil {
		t.Fatalf("expected target replicas validation error")
	}
}

func TestCreateComponentDoesNotReenterRollingAfterPromotion(t *testing.T) {
	const compName = "comp"

	transformer := &rolloutCreateTransformer{}
	rollout := &appsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("12345678"),
		},
		Status: appsv1alpha1.RolloutStatus{
			Components: []appsv1alpha1.RolloutComponentStatus{
				{
					Name:                   compName,
					Replicas:               2,
					CanaryReplicas:         1,
					LastScaleUpTimestamp:   metav1.Now(),
					LastScaleDownTimestamp: metav1.Now(),
				},
			},
		},
	}
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	spec := &appsv1.ClusterComponentSpec{
		Replicas: 2,
		Instances: []appsv1.InstanceTemplate{
			{
				Name:     prefix,
				Canary:   ptr.To(false),
				Replicas: ptr.To[int32](1),
			},
		},
	}
	comp := appsv1alpha1.RolloutComponent{
		Name:     compName,
		Replicas: ptr.To(intstr.FromInt32(1)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{
				Canary: ptr.To(true),
				Promotion: &appsv1alpha1.RolloutPromotion{
					Auto:                  ptr.To(true),
					DelaySeconds:          ptr.To[int32](0),
					ScaleDownDelaySeconds: ptr.To[int32](0),
				},
			},
		},
	}
	transCtx := &rolloutTransformContext{
		Rollout: rollout,
		ClusterOrig: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: appsv1.ClusterStatus{
				ObservedGeneration: 1,
				Components: map[string]appsv1.ClusterComponentStatus{
					compName: {
						Phase: appsv1.RunningComponentPhase,
					},
				},
			},
		},
		ClusterComps: map[string]*appsv1.ClusterComponentSpec{
			compName: spec,
		},
		Components: map[string]*appsv1.Component{
			compName: {
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Status: appsv1.ComponentStatus{
					ObservedGeneration: 1,
					Phase:              appsv1.RunningComponentPhase,
				},
			},
		},
	}

	if err := transformer.component(transCtx, rollout, comp); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.Replicas != 2 {
		t.Fatalf("promoted create rollout re-entered rolling, replicas = %d, want 2", spec.Replicas)
	}
	if got := ptr.Deref(spec.Instances[0].Replicas, 0); got != 1 {
		t.Fatalf("unexpected promoted template replicas: %d", got)
	}
}

func TestCreateComponentReentersRollingWhenPromotedTemplateTargetChanges(t *testing.T) {
	const compName = "comp"

	transformer := &rolloutCreateTransformer{}
	rollout := &appsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("12345678"),
		},
		Status: appsv1alpha1.RolloutStatus{
			Components: []appsv1alpha1.RolloutComponentStatus{
				{
					Name:                   compName,
					Replicas:               2,
					CanaryReplicas:         1,
					LastScaleUpTimestamp:   metav1.Now(),
					LastScaleDownTimestamp: metav1.Now(),
				},
			},
		},
	}
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	spec := &appsv1.ClusterComponentSpec{
		Replicas: 2,
		Instances: []appsv1.InstanceTemplate{
			{
				Name:     prefix,
				Canary:   ptr.To(false),
				Replicas: ptr.To[int32](1),
			},
		},
	}
	comp := appsv1alpha1.RolloutComponent{
		Name:     compName,
		Replicas: ptr.To(intstr.FromInt32(2)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{
				Canary: ptr.To(true),
				Promotion: &appsv1alpha1.RolloutPromotion{
					Auto:                  ptr.To(true),
					DelaySeconds:          ptr.To[int32](0),
					ScaleDownDelaySeconds: ptr.To[int32](0),
				},
			},
		},
	}
	transCtx := &rolloutTransformContext{
		Rollout: rollout,
		ClusterOrig: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: appsv1.ClusterStatus{
				ObservedGeneration: 1,
				Components: map[string]appsv1.ClusterComponentStatus{
					compName: {
						Phase: appsv1.RunningComponentPhase,
					},
				},
			},
		},
		ClusterComps: map[string]*appsv1.ClusterComponentSpec{
			compName: spec,
		},
		Components: map[string]*appsv1.Component{
			compName: {
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Status: appsv1.ComponentStatus{
					ObservedGeneration: 1,
					Phase:              appsv1.RunningComponentPhase,
				},
			},
		},
	}

	if err := transformer.component(transCtx, rollout, comp); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.Replicas != 4 {
		t.Fatalf("expected changed target to re-enter rolling, replicas = %d, want 4", spec.Replicas)
	}
	if got := ptr.Deref(spec.Instances[0].Replicas, 0); got != 2 {
		t.Fatalf("expected promoted template target to be updated, got %d", got)
	}
}

func TestCreateShardingDoesNotReenterRollingAfterPromotion(t *testing.T) {
	const shardingName = "sharding"

	transformer := &rolloutCreateTransformer{}
	rollout := &appsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("12345678"),
		},
		Status: appsv1alpha1.RolloutStatus{
			Shardings: []appsv1alpha1.RolloutShardingStatus{
				{
					Name:                   shardingName,
					Replicas:               2,
					CanaryReplicas:         1,
					LastScaleUpTimestamp:   metav1.Now(),
					LastScaleDownTimestamp: metav1.Now(),
				},
			},
		},
	}
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	spec := &appsv1.ClusterSharding{
		Name:   shardingName,
		Shards: 1,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 2,
			Instances: []appsv1.InstanceTemplate{
				{
					Name:     prefix,
					Canary:   ptr.To(false),
					Replicas: ptr.To[int32](1),
				},
			},
		},
	}
	sharding := appsv1alpha1.RolloutSharding{
		Name:     shardingName,
		Replicas: ptr.To(intstr.FromInt32(1)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{
				Canary: ptr.To(true),
				Promotion: &appsv1alpha1.RolloutPromotion{
					Auto:                  ptr.To(true),
					DelaySeconds:          ptr.To[int32](0),
					ScaleDownDelaySeconds: ptr.To[int32](0),
				},
			},
		},
	}
	transCtx := &rolloutTransformContext{
		Rollout: rollout,
		ClusterOrig: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: appsv1.ClusterStatus{
				ObservedGeneration: 1,
				Shardings: map[string]appsv1.ClusterShardingStatus{
					shardingName: {
						Phase: appsv1.RunningComponentPhase,
					},
				},
			},
		},
		ClusterShardings: map[string]*appsv1.ClusterSharding{
			shardingName: spec,
		},
		ShardingComps: map[string][]*appsv1.Component{
			shardingName: {
				{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: appsv1.ComponentStatus{
						ObservedGeneration: 1,
						Phase:              appsv1.RunningComponentPhase,
					},
				},
			},
		},
	}

	if err := transformer.sharding(transCtx, rollout, sharding); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.Template.Replicas != 2 {
		t.Fatalf("promoted create sharding rollout re-entered rolling, replicas = %d, want 2", spec.Template.Replicas)
	}
	if got := ptr.Deref(spec.Template.Instances[0].Replicas, 0); got != 1 {
		t.Fatalf("unexpected promoted template replicas: %d", got)
	}
}

func TestCreateShardingReentersRollingWhenPromotedTemplateTargetChanges(t *testing.T) {
	const shardingName = "sharding"

	transformer := &rolloutCreateTransformer{}
	rollout := &appsv1alpha1.Rollout{
		ObjectMeta: metav1.ObjectMeta{
			UID: types.UID("12345678"),
		},
		Status: appsv1alpha1.RolloutStatus{
			Shardings: []appsv1alpha1.RolloutShardingStatus{
				{
					Name:                   shardingName,
					Replicas:               2,
					CanaryReplicas:         1,
					LastScaleUpTimestamp:   metav1.Now(),
					LastScaleDownTimestamp: metav1.Now(),
				},
			},
		},
	}
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	spec := &appsv1.ClusterSharding{
		Name:   shardingName,
		Shards: 1,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 2,
			Instances: []appsv1.InstanceTemplate{
				{
					Name:     prefix,
					Canary:   ptr.To(false),
					Replicas: ptr.To[int32](1),
				},
			},
		},
	}
	sharding := appsv1alpha1.RolloutSharding{
		Name:     shardingName,
		Replicas: ptr.To(intstr.FromInt32(2)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{
				Canary: ptr.To(true),
				Promotion: &appsv1alpha1.RolloutPromotion{
					Auto:                  ptr.To(true),
					DelaySeconds:          ptr.To[int32](0),
					ScaleDownDelaySeconds: ptr.To[int32](0),
				},
			},
		},
	}
	transCtx := &rolloutTransformContext{
		Rollout: rollout,
		ClusterOrig: &appsv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Generation: 1,
			},
			Status: appsv1.ClusterStatus{
				ObservedGeneration: 1,
				Shardings: map[string]appsv1.ClusterShardingStatus{
					shardingName: {
						Phase: appsv1.RunningComponentPhase,
					},
				},
			},
		},
		ClusterShardings: map[string]*appsv1.ClusterSharding{
			shardingName: spec,
		},
		ShardingComps: map[string][]*appsv1.Component{
			shardingName: {
				{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: appsv1.ComponentStatus{
						ObservedGeneration: 1,
						Phase:              appsv1.RunningComponentPhase,
					},
				},
			},
		},
	}

	if err := transformer.sharding(transCtx, rollout, sharding); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if spec.Template.Replicas != 4 {
		t.Fatalf("expected changed target to re-enter rolling, replicas = %d, want 4", spec.Template.Replicas)
	}
	if got := ptr.Deref(spec.Template.Instances[0].Replicas, 0); got != 2 {
		t.Fatalf("expected promoted template target to be updated, got %d", got)
	}
}

func TestRolloutSchedulingPolicyCopiesFields(t *testing.T) {
	policy := &appsv1.SchedulingPolicy{
		SchedulerName: "custom-scheduler",
		NodeSelector: map[string]string{
			"disk": "ssd",
		},
		NodeName: "node-a",
		Tolerations: []corev1.Toleration{
			{
				Key:      "dedicated",
				Operator: corev1.TolerationOpEqual,
				Value:    "db",
				Effect:   corev1.TaintEffectNoSchedule,
			},
		},
	}

	got := rolloutSchedulingPolicy(policy)
	if got == nil {
		t.Fatalf("expected scheduling policy to be copied")
		return
	}
	if got.SchedulerName != policy.SchedulerName {
		t.Fatalf("unexpected scheduler name: %s", got.SchedulerName)
	}
	if got.NodeSelector["disk"] != "ssd" {
		t.Fatalf("unexpected node selector copy: %#v", got.NodeSelector)
	}
	if got.NodeName != policy.NodeName {
		t.Fatalf("unexpected node name: %s", got.NodeName)
	}
	if len(got.Tolerations) != 1 || got.Tolerations[0].Key != "dedicated" {
		t.Fatalf("unexpected tolerations copy: %#v", got.Tolerations)
	}
}

func TestCreateShardingReplicasDefaultsToNoOpWhenUnset(t *testing.T) {
	rollout := &appsv1alpha1.Rollout{
		Status: appsv1alpha1.RolloutStatus{
			Shardings: []appsv1alpha1.RolloutShardingStatus{
				{
					Name:     "sharding",
					Replicas: 6,
				},
			},
		},
	}
	sharding := appsv1alpha1.RolloutSharding{
		Name: "sharding",
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{},
		},
	}
	spec := &appsv1.ClusterSharding{
		Shards: 2,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 3,
		},
	}

	replicas, targetReplicas, err := createShardingReplicas(rollout, sharding, spec)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if replicas != 3 {
		t.Fatalf("unexpected original replicas: %d", replicas)
	}
	if targetReplicas != 0 {
		t.Fatalf("expected zero target replicas when sharding.replicas is unset, got %d", targetReplicas)
	}
}

func TestCreateShardingReplicasRejectsTargetGreaterThanOriginal(t *testing.T) {
	rollout := &appsv1alpha1.Rollout{
		Status: appsv1alpha1.RolloutStatus{
			Shardings: []appsv1alpha1.RolloutShardingStatus{
				{
					Name:     "sharding",
					Replicas: 3,
				},
			},
		},
	}
	sharding := appsv1alpha1.RolloutSharding{
		Name:     "sharding",
		Replicas: ptr.To(intstr.FromInt32(4)),
		Strategy: appsv1alpha1.RolloutStrategy{
			Create: &appsv1alpha1.RolloutStrategyCreate{},
		},
	}
	spec := &appsv1.ClusterSharding{
		Shards: 1,
		Template: appsv1.ClusterComponentSpec{
			Replicas: 3,
		},
	}

	_, _, err := createShardingReplicas(rollout, sharding, spec)
	if err == nil {
		t.Fatalf("expected target replicas validation error")
	}
}
