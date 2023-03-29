/*
Copyright ApeCloud, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package lifecycle

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("transformer component utils test", func() {
	Context("test mergeServiceAnnotations", func() {
		It("original and target annotations are nil", func() {
			Expect(mergeServiceAnnotations(nil, nil)).Should(BeNil())
		})
		It("target annotations is nil", func() {
			originalAnnotations := map[string]string{"k1": "v1"}
			Expect(mergeServiceAnnotations(originalAnnotations, nil)).To(Equal(originalAnnotations))
		})
		It("original annotations is nil", func() {
			targetAnnotations := map[string]string{"k1": "v1"}
			Expect(mergeServiceAnnotations(nil, targetAnnotations)).To(Equal(targetAnnotations))
		})
		It("original annotations have prometheus annotations which should be removed", func() {
			originalAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			targetAnnotations := map[string]string{"k2": "v2"}
			expectAnnotations := map[string]string{"k1": "v1", "k2": "v2"}
			Expect(mergeServiceAnnotations(originalAnnotations, targetAnnotations)).To(Equal(expectAnnotations))
		})
		It("target annotations should override original annotations", func() {
			originalAnnotations := map[string]string{"k1": "v1", "prometheus.io/path": "/metrics"}
			targetAnnotations := map[string]string{"k1": "v11"}
			expectAnnotations := map[string]string{"k1": "v11"}
			Expect(mergeServiceAnnotations(originalAnnotations, targetAnnotations)).To(Equal(expectAnnotations))
		})
	})
})
