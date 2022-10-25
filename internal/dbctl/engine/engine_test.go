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
		Expect(engine.GetEngineName()).Should(Equal(mysqlEngineName))

		url := engine.GetConnectURL("test")
		Expect(len(url)).Should(Equal(3))

		url = engine.GetConnectURL("")
		Expect(len(url)).Should(Equal(1))

		Expect(engine.GetEngineContainer()).Should(Equal(mysqlContainerName))
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := New(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})
})
