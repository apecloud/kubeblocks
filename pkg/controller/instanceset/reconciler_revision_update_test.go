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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	workloads "github.com/apecloud/kubeblocks/apis/workloads/v1"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/instancetemplate"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/rollingupdate"
)

var _ = Describe("revision update reconciler test", func() {
	BeforeEach(func() {
		its = builder.NewInstanceSetBuilder(namespace, name).
			SetReplicas(3).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			its.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(its)
			reconciler := NewRevisionUpdateReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ConditionSatisfied))

			By("Reconcile")
			res, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			Expect(res).Should(Equal(kubebuilderx.Continue))
			newITS, ok := tree.GetRoot().(*workloads.InstanceSet)
			Expect(ok).Should(BeTrue())
			Expect(newITS.Status.ObservedGeneration).Should(Equal(its.Generation))
			updateRevisions, err := GetRevisions(newITS.Status.UpdateRevisions)
			Expect(err).Should(BeNil())
			Expect(updateRevisions).Should(HaveLen(3))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-0"))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-1"))
			Expect(updateRevisions).Should(HaveKey(its.Name + "-2"))
			Expect(newITS.Status.UpdateRevision).Should(Equal(updateRevisions[its.Name+"-2"]))
			rolloutID := newITS.Annotations[rollingupdate.RolloutIDAnnotationKey]
			Expect(rolloutID).ShouldNot(BeEmpty())

			By("preserving the rollout ID for a name-set-only change")
			*its.Spec.Replicas = 4
			_, err = reconciler.Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(newITS.Annotations[rollingupdate.RolloutIDAnnotationKey]).Should(Equal(rolloutID))
			scaledRevisions, err := GetRevisions(newITS.Status.UpdateRevisions)
			Expect(err).ShouldNot(HaveOccurred())

			By("advancing the rollout ID for a filtered in-place update")
			its.Spec.Template.Spec.Containers[0].Image += "-changed"
			_, err = reconciler.Reconcile(tree)
			Expect(err).ShouldNot(HaveOccurred())
			updatedRevisions, err := GetRevisions(newITS.Status.UpdateRevisions)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(updatedRevisions).Should(Equal(scaledRevisions))
			Expect(newITS.Annotations[rollingupdate.RolloutIDAnnotationKey]).ShouldNot(Equal(rolloutID))
		})

		It("detects only surviving flat ordinal reassignment", func() {
			previous := map[string]workloads.Ordinals{
				"a": {Discrete: []int32{0}},
				"b": {Discrete: []int32{1}},
			}
			Expect(hasReassignedOrdinal(previous, map[string]*instancetemplate.InstanceTemplateExt{
				"test-0": {Name: "a"},
				"test-1": {Name: "b"},
				"test-2": {Name: "b"},
			})).Should(BeFalse())
			Expect(hasReassignedOrdinal(previous, map[string]*instancetemplate.InstanceTemplateExt{
				"test-0": {Name: "b"},
				"test-1": {Name: "a"},
			})).Should(BeTrue())
		})
	})
})
