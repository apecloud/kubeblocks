/*
Copyright ApeCloud Inc.

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

package engine

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("playground", func() {
	It("new engine", func() {
		engine, err := New("wesql")
		Expect(engine).ShouldNot(BeNil())
		Expect(engine.Install(3, "test", "test")).ShouldNot(BeNil())
		Expect(err).Should(BeNil())

		engine, err = New("test")
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())

		_, err = componentPath(3)
		Expect(err).Should(Or(Succeed(), HaveOccurred()))

		options, err := newCreateOptions("test", "test", "")
		Expect(options).ShouldNot(BeNil())
		Expect(err).Should(Succeed())
	})
})
