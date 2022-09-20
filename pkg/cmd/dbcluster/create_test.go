package dbcluster

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

var _ = Describe("Create", func() {
	var streams genericclioptions.IOStreams

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	It("without name", func() {
		o := &CreateOptions{IOStreams: streams}
		Expect(o.Validate([]string{})).To(MatchError("missing cluster name"))
	})

	It("without cluster definition", func() {
		o := &CreateOptions{IOStreams: streams}
		Expect(o.Validate([]string{"test"})).To(MatchError("cluster-definition can not be empty"))
	})

	It("without app-version", func() {
		o := &CreateOptions{
			IOStreams:     streams,
			ClusterDefRef: "wesql",
		}
		Expect(o.Validate([]string{"test"})).To(MatchError("app-version can not be empty"))
	})

	It("run", func() {
		tf := cmdtesting.NewTestFactory().WithNamespace("default")
		defer tf.Cleanup()
		tf.ClientConfigVal = cfg

		o := &CreateOptions{
			IOStreams:     streams,
			ClusterDefRef: "wesql",
			AppVersionRef: "app-version",
			Components:    "",
		}
		o.Complete(tf, []string{"test"})
		Expect(o.Namespace).To(Equal("default"))
		Expect(o.Name).To(Equal("test"))

		Expect(o.Run()).Should(Succeed())

		del := &DeleteOptions{}
		del.Complete(tf, []string{"test"})
		Expect(del.Namespace).To(Equal("default"))
		Expect(del.Run()).Should(Succeed())
	})
})
