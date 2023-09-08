package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("cluster register", func() {
	var streams genericclioptions.IOStreams
	var tf *cmdtesting.TestFactory

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = cmdtesting.NewTestFactory().WithNamespace("default")
	})

	It("register command", func() {
		option := newRegisterOption(tf, streams)
		Expect(option).ShouldNot(BeNil())

		cmd := newRegisterCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

	It("register command validate", func() {
		o := &registerOption{
			Factory:     tf,
			IOStreams:   streams,
			clusterType: "not-allow-name",
		}
		Expect(o.validate()).Should(HaveOccurred())

		o.clusterType = "mysql"
		// already exist
		Expect(o.validate()).Should(HaveOccurred())

		o.clusterType = "oracle"
		Expect(o.validate()).Should(Succeed())
	})
})
