/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package rsm2

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/kubebuilderx"
	"github.com/apecloud/kubeblocks/pkg/controller/model"
)

var _ = Describe("assistant object reconciler test", func() {
	BeforeEach(func() {
		rsm = builder.NewReplicatedStateMachineBuilder(namespace, name).
			SetUID(uid).
			SetReplicas(3).
			AddMatchLabelsInMap(selectors).
			SetTemplate(template).
			SetVolumeClaimTemplates(volumeClaimTemplates...).
			SetRoles(roles).
			GetObject()
	})

	Context("PreCondition & Reconcile", func() {
		It("should work well", func() {
			By("PreCondition")
			rsm.Generation = 1
			tree := kubebuilderx.NewObjectTree()
			tree.SetRoot(rsm)
			reconciler = NewAssistantObjectReconciler()
			Expect(reconciler.PreCondition(tree)).Should(Equal(kubebuilderx.ResultSatisfied))

			By("do reconcile")
			_, err := reconciler.Reconcile(tree)
			Expect(err).Should(BeNil())
			// desired: svc: "bar-headless", cm: "bar"
			objects := tree.GetSecondaryObjects()
			Expect(objects).Should(HaveLen(2))
			svc := builder.NewHeadlessServiceBuilder(namespace, name+"-headless").GetObject()
			cm := builder.NewConfigMapBuilder(namespace, name+"-rsm-env").GetObject()
			for _, object := range []client.Object{svc, cm} {
				name, err := model.GetGVKName(object)
				Expect(err).Should(BeNil())
				_, ok := objects[*name]
				Expect(ok).Should(BeTrue())
			}
		})
	})
})
