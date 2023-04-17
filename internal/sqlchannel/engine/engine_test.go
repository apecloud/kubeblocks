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

package engine

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Engine", func() {
	It("new mysql engine", func() {
		for _, typeName := range []string{stateMysql, statePostgreSQL, stateRedis} {
			engine, _ := New(typeName)
			Expect(engine).ShouldNot(BeNil())

			url := engine.ConnectCommand(nil)
			Expect(len(url)).Should(Equal(3))

			url = engine.ConnectCommand(nil)
			Expect(len(url)).Should(Equal(3))
			// it is a tricky way to check the container name
			// for the moment, we only support mysql, postgresql and redis
			// and the container name is the same as the state name
			Expect(engine.Container()).Should(Equal(typeName))
		}
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := New(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})
})
