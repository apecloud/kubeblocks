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

		info, err := engine.GetExecInfo("connect")
		Expect(info).ShouldNot(BeNil())
		Expect(err).ShouldNot(HaveOccurred())

		info, err = engine.GetExecInfo("test")
		Expect(info).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})

	It("new unknown engine", func() {
		typeName := "unknown-type"
		engine, err := New(typeName)
		Expect(engine).Should(BeNil())
		Expect(err).Should(HaveOccurred())
	})
})
