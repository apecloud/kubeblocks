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

package restore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	dpv1alpha1 "github.com/apecloud/kubeblocks/apis/dataprotection/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/constant"
)

func TestGetSourcePodNameForTargetPod(t *testing.T) {
	target := &dpv1alpha1.BackupStatusTarget{BackupTarget: dpv1alpha1.BackupTarget{PodSelector: &dpv1alpha1.PodSelector{
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
			constant.AppInstanceLabelKey: "source", constant.KBAppComponentLabelKey: "redis",
		}}, Strategy: dpv1alpha1.PodSelectionStrategyAll,
	}}}
	policy := &dpv1alpha1.RequiredPolicyForAllPodSelection{DataRestorePolicy: dpv1alpha1.OneToOneRestorePolicy}

	t.Run("matches instance template identity without list order", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-a-4", "source-redis-az-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-az-a-3", podName)
	})

	t.Run("disambiguates templates sharing an ordinal", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-b-3", "source-redis-az-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-az-a-3", podName)
	})

	t.Run("matches the exact template rather than a suffix", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-b-a-3", "source-redis-a-3"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-a-3", "a")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-a-3", podName)
	})

	t.Run("does not treat a flat workload suffix as a template", func(t *testing.T) {
		target.PodSelector.MatchLabels[constant.KBAppComponentLabelKey] = "redis-a"
		target.SelectedTargetPods = []string{"source-redis-a-3"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-a-3", "a")
		assert.ErrorContains(t, err, "no selected source target pod matches")
		target.PodSelector.MatchLabels[constant.KBAppComponentLabelKey] = "redis"
	})

	t.Run("matches a flat ordinal with holes", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-7", "source-redis-2"}
		podName, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-7", "")
		assert.NoError(t, err)
		assert.Equal(t, "source-redis-7", podName)
	})

	t.Run("rejects an ambiguous generic ordinal", func(t *testing.T) {
		labelSelector := target.PodSelector.LabelSelector
		target.PodSelector.LabelSelector = nil
		defer func() { target.PodSelector.LabelSelector = labelSelector }()
		target.SelectedTargetPods = []string{"source-redis-az-a-3", "source-redis-az-b-3"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-3", "")
		assert.ErrorContains(t, err, "multiple selected source target pods")
	})

	t.Run("rejects a missing identity", func(t *testing.T) {
		target.SelectedTargetPods = []string{"source-redis-az-a-4"}
		_, err := GetSourcePodNameForTargetPod(target, policy, "target-redis-az-a-3", "az-a")
		assert.ErrorContains(t, err, "no selected source target pod matches")
	})
}
