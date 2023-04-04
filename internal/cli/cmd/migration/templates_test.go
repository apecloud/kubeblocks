package migration

import (
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("templates", func() {

	var (
		streams genericclioptions.IOStreams
		tf      *cmdtesting.TestFactory
	)

	It("command build", func() {
		cmd := NewMigrationTemplatesCmd(tf, streams)
		Expect(cmd).ShouldNot(BeNil())
	})

})
