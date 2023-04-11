package testing

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("test fake in internal/preflight", func() {
	It("test FakeKbPreflight", func() {
		kbPreflight := FakeKbPreflight()
		Expect(kbPreflight).ShouldNot(BeNil())
		Expect(len(kbPreflight.Spec.Collectors)).Should(BeNumerically(">", 0))
	})

	It("test FakeKbHostPreflight", func() {
		hostKbPreflight := FakeKbHostPreflight()
		Expect(hostKbPreflight).ShouldNot(BeNil())
		Expect(len(hostKbPreflight.Spec.RemoteCollectors)).Should(BeNumerically(">", 0))
		Expect(len(hostKbPreflight.Spec.ExtendCollectors)).Should(BeNumerically(">", 0))
	})

	It("test FakeAnalyzers", func() {
		analyzers := FakeAnalyzers()
		Expect(analyzers).ShouldNot(BeNil())
		Expect(len(analyzers)).Should(BeNumerically(">", 0))
	})
})
