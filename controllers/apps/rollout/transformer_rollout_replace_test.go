/*
Copyright (C) 2022-2025 ApeCloud Co., Ltd

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
