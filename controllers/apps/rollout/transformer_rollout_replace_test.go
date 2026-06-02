/*
Copyright (C) 2022-2026 ApeCloud Co., Ltd

# This file is part of KubeBlocks project

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

	"k8s.io/apimachinery/pkg/types"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
)

func TestReplaceCompInstanceTemplatesFromSpecSupportsLegacyPrefix(t *testing.T) {
	rollout := &appsv1alpha1.Rollout{}
	rollout.UID = types.UID("12345678-1234-1234-1234-1234567890ab")
	prefix := replaceInstanceTemplateNamePrefix(rollout)

	spec := &appsv1.ClusterComponentSpec{
		Instances: []appsv1.InstanceTemplate{
			{Name: "aaa"},
			{Name: prefix},
			{Name: prefix + "-aaa"},
		},
	}

	tpls := replaceCompInstanceTemplatesFromSpec(rollout, spec)
	if len(tpls) != 2 {
		t.Fatalf("expected 2 rollout templates, got %d", len(tpls))
	}
	if got := tpls[""].Name; got != prefix {
		t.Fatalf("expected default legacy template %q, got %q", prefix, got)
	}
	if got := tpls["aaa"].Name; got != prefix+"-aaa" {
		t.Fatalf("expected instance legacy template %q, got %q", prefix+"-aaa", got)
	}
}

func TestReplaceShardingInstanceTemplatesFromSpecSupportsLegacyPrefix(t *testing.T) {
	rollout := &appsv1alpha1.Rollout{}
	rollout.UID = types.UID("12345678-1234-1234-1234-1234567890ab")
	prefix := replaceInstanceTemplateNamePrefix(rollout)

	spec := &appsv1.ClusterSharding{
		Template: appsv1.ClusterComponentSpec{
			Instances: []appsv1.InstanceTemplate{
				{Name: "aaa"},
				{Name: prefix},
				{Name: prefix + "-aaa"},
			},
		},
	}

	tpls := replaceShardingInstanceTemplatesFromSpec(rollout, spec)
	if len(tpls) != 2 {
		t.Fatalf("expected 2 rollout templates, got %d", len(tpls))
	}
	if got := tpls[""].Name; got != prefix {
		t.Fatalf("expected default legacy template %q, got %q", prefix, got)
	}
	if got := tpls["aaa"].Name; got != prefix+"-aaa" {
		t.Fatalf("expected instance legacy template %q, got %q", prefix+"-aaa", got)
	}
}

func TestIsRolloutManagedInstanceTemplateNameSupportsLegacyAndNewNames(t *testing.T) {
	rollout := &appsv1alpha1.Rollout{}
	rollout.UID = types.UID("12345678-1234-1234-1234-1234567890ab")
	prefix := replaceInstanceTemplateNamePrefix(rollout)
	suffix := replaceInstanceTemplateNameSuffix(rollout)

	cases := map[string]bool{
		"":              false,
		"aaa":           false,
		prefix:          true,
		prefix + "-aaa": true,
		suffix:          true,
		"aaa-" + suffix: true,
	}
	for name, expected := range cases {
		if got := isRolloutManagedInstanceTemplateName(rollout, name); got != expected {
			t.Fatalf("name %q expected %v, got %v", name, expected, got)
		}
	}
}

// TestIsRolloutManagedInstanceTemplateRejectsForeignRolloutCanaries verifies
// that templates created by a different (previous) Rollout — which still carry
// the "created-by-rollout" annotation but whose names reference that previous
// Rollout's UID — are not treated as managed by the current Rollout.
//
// Prior to the fix, isRolloutManagedInstanceTemplate also matched on the
// annotation alone, so leftover canary templates from a completed rollout were
// misclassified as managed by every subsequent rollout. That misclassification
// caused replaceShardingInstanceTemplatesFromSpec to return a non-empty map
// without a default entry, which made tpls[""].DeepCopy() in the teardown
// transformer dereference a nil pointer and panic the controller manager.
func TestIsRolloutManagedInstanceTemplateRejectsForeignRolloutCanaries(t *testing.T) {
	previous := &appsv1alpha1.Rollout{}
	previous.UID = types.UID("11111111-1111-1111-1111-111111111111")
	current := &appsv1alpha1.Rollout{}
	current.UID = types.UID("22222222-2222-2222-2222-222222222222")

	previousCanary := appsv1.InstanceTemplate{
		Name: replaceInstanceTemplateNameSuffix(previous),
		Annotations: map[string]string{
			instanceTemplateCreatedByAnnotationKey: "yes",
		},
	}
	if !isRolloutManagedInstanceTemplate(previous, previousCanary) {
		t.Fatalf("previous rollout should manage its own canary")
	}
	if isRolloutManagedInstanceTemplate(current, previousCanary) {
		t.Fatalf("current rollout must not claim a canary created by a previous rollout")
	}
}

// TestReplaceShardingInstanceTemplatesAlwaysProvidesDefaultForFreshRollout
// reproduces the consecutive-rollout panic. The cluster sharding spec contains
// a canary template left behind by a successful prior rollout (named with the
// previous rollout's UID, annotation set). A fresh rollout then asks for its
// template map. Before the fix, replaceShardingInstanceTemplatesFromSpec
// matched the leftover via the annotation, the early-return path fired with
// only a non-default entry, and the caller saw tpls[""] == nil — exactly the
// state that crashed the teardown transformer at tpls[""].DeepCopy().
func TestReplaceShardingInstanceTemplatesAlwaysProvidesDefaultForFreshRollout(t *testing.T) {
	previous := &appsv1alpha1.Rollout{}
	previous.UID = types.UID("11111111-1111-1111-1111-111111111111")
	current := &appsv1alpha1.Rollout{}
	current.UID = types.UID("22222222-2222-2222-2222-222222222222")
	current.Spec.Shardings = []appsv1alpha1.RolloutSharding{{
		Name: "shard",
		Strategy: appsv1alpha1.RolloutStrategy{
			Replace: &appsv1alpha1.RolloutStrategyReplace{},
		},
	}}

	replicas := int32(3)
	spec := &appsv1.ClusterSharding{
		Name: "shard",
		Template: appsv1.ClusterComponentSpec{
			Replicas:            replicas,
			FlatInstanceOrdinal: true,
			Instances: []appsv1.InstanceTemplate{
				{
					Name:     replaceInstanceTemplateNameSuffix(previous),
					Replicas: &replicas,
					Annotations: map[string]string{
						instanceTemplateCreatedByAnnotationKey: "yes",
					},
				},
			},
		},
	}

	tpls, exist, err := replaceShardingInstanceTemplates(current, current.Spec.Shardings[0], spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exist {
		t.Fatalf("expected exist=false (no template managed by current rollout), got true")
	}
	if tpls[""] == nil {
		t.Fatalf("expected a default template entry for the current rollout, got nil — this is the regression that panics teardown")
	}
}
