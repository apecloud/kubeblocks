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

var _ = Describe("Engine", func() {
	It("new mysql engine", func() {
		typeName := stateMysql
		engine, _ := New(typeName)
		Expect(engine).ShouldNot(BeNil())
		Expect(engine.EngineName()).Should(Equal(mysqlEngineName))

		url := engine.ConnectCommand("test")
		Expect(len(url)).Should(Equal(3))

		url = engine.ConnectCommand("")
		Expect(len(url)).Should(Equal(1))

		Expect(engine.EngineContainer()).Should(Equal(mysqlContainerName))
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := New(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})
	Context("DataEngines Test", func() {
		BeforeEach(func() {
			// clear DataEngines registry
			DataEngines = make(map[string]map[string]interface{}, 0)
		})
		It("Registry Test", func() {
			Registry(stateMysql, connectModule, "test")
			Expect(len(DataEngines)).To(Equal(1))
			Expect(GetContext(stateMysql, connectModule)).To(Equal("test"))
		})

		It("GetContext Test", func() {
			_, err := GetContext(stateMysql, connectModule)
			Expect(err).To(MatchError("no registered data engine " + stateMysql))
			Registry(stateMysql, connectModule, "test")
			_, err = GetContext(stateMysql, connectModule+"test")
			Expect(err).To(MatchError("no registered context for module " + connectModule + "test"))
		})

	})
})
