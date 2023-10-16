package clusterdefinition

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
)

// todo: wait #https://github.com/apecloud/kubeblocks/pull/5422 merge main and fix it

var _ = Describe("clusterdefinition list components", func() {
	var (
		streams genericiooptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	It("create list-components cmd", func() {
		cmd := NewListComponentsCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

})
