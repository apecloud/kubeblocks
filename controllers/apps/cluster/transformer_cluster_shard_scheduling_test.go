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

package cluster

import (
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	"github.com/apecloud/kubeblocks/pkg/controller/scheduling"
	testapps "github.com/apecloud/kubeblocks/pkg/testutil/apps"
)

func shardSchedulingPolicyForTest(compName string) *appsv1.SchedulingPolicy {
	term := corev1.PodAffinityTerm{
		TopologyKey: corev1.LabelHostname,
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
			constant.KBAppComponentLabelKey: compName,
		}},
	}
	return &appsv1.SchedulingPolicy{Affinity: &corev1.Affinity{
		PodAffinity: &corev1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  []corev1.PodAffinityTerm{*term.DeepCopy()},
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 50, PodAffinityTerm: *term.DeepCopy()}},
		},
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution:  []corev1.PodAffinityTerm{*term.DeepCopy()},
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 50, PodAffinityTerm: *term.DeepCopy()}},
		},
	}}
}

func shardSelectorsWithoutPlaceholdersForTest() *appsv1.SchedulingPolicy {
	return &appsv1.SchedulingPolicy{Affinity: &corev1.Affinity{PodAntiAffinity: &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{TopologyKey: corev1.LabelHostname}},
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{Weight: 50, PodAffinityTerm: corev1.PodAffinityTerm{
			TopologyKey: corev1.LabelHostname,
			LabelSelector: &metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: constant.KBAppComponentLabelKey, Operator: metav1.LabelSelectorOpExists},
			}},
		}}},
	}}}
}

func expectShardInstanceSchedulingForTest(comp *appsv1.Component, cli client.Reader) {
	compDef := testapps.NewComponentDefinitionFactory(comp.Spec.CompDef).SetRuntime(nil).GetObject()
	synthesized, err := component.BuildSynthesizedComponent(ctx, cli, compDef, comp)
	Expect(err).ShouldNot(HaveOccurred())
	for _, instance := range comp.Spec.Instances {
		podSpec := synthesized.PodSpec.DeepCopy()
		scheduling.ApplySchedulingPolicyToPodSpec(podSpec, instance.SchedulingPolicy)
		compName := comp.Labels[constant.KBAppComponentLabelKey]
		if instance.Name == "explicit" {
			compName = "other-component"
		}
		Expect(podSpec.Affinity).Should(Equal(shardSchedulingPolicyForTest(compName).Affinity))
	}
}
