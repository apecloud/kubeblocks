/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

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

package lifecycle

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("cluster plan utils test", func() {
	Context("test mergeServiceAnnotations", func() {
		It("target annotations is nil", func() {
			originalAnnotations := map[string]string{"k1": "v1"}
			var targetAnnotations map[string]string
			expectAnnotations := map[string]string{"k1": "v1"}
			mergeServiceAnnotations(originalAnnotations, &targetAnnotations)
			Expect(targetAnnotations).To(Equal(expectAnnotations))
		})
		It("original annotations is nil", func() {
			targetAnnotations := map[string]string{"k1": "v1"}
			expectAnnotations := map[string]string{"k1": "v1"}
			mergeServiceAnnotations(nil, &targetAnnotations)
			Expect(targetAnnotations).To(Equal(expectAnnotations))
		})
		It("target annotations have prometheus annotations which should be removed", func() {
			originalAnnotations := map[string]string{"k2": "v2"}
			targetAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			expectAnnotations := map[string]string{"k1": "v1", "k2": "v2"}
			mergeServiceAnnotations(originalAnnotations, &targetAnnotations)
			Expect(targetAnnotations).To(Equal(expectAnnotations))
		})
		It("original and target annotations both have prometheus annotations", func() {
			originalAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			targetAnnotations := map[string]string{"k1": "v11", "prometheus.io/path": "/metrics2"}
			expectAnnotations := map[string]string{"k1": "v11", "prometheus.io/path": "/metrics"}
			mergeServiceAnnotations(originalAnnotations, &targetAnnotations)
			Expect(targetAnnotations).To(Equal(expectAnnotations))
		})

		It("should merge annotations from original that not exist in target to final result", func() {
			originalKey := "only-existing-in-original"
			targetKey := "only-existing-in-target"
			updatedKey := "updated-in-target"
			originalAnnotations := map[string]string{
				originalKey: "true",
				updatedKey:  "false",
			}
			targetAnnotations := map[string]string{
				targetKey:  "true",
				updatedKey: "true",
			}
			mergeAnnotations(originalAnnotations, &targetAnnotations)
			Expect(targetAnnotations[targetKey]).ShouldNot(BeEmpty())
			Expect(targetAnnotations[originalKey]).ShouldNot(BeEmpty())
			Expect(targetAnnotations[updatedKey]).Should(Equal("true"))
			By("merging with target being nil")
			var nilAnnotations map[string]string
			mergeAnnotations(originalAnnotations, &nilAnnotations)
			Expect(nilAnnotations).ShouldNot(BeNil())
		})
	})
})
