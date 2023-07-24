package context

import (
	ginkgo_context "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

type MockContext struct {
	genericclioptions.IOStreams
}

func (m *MockContext) showContext() error {
	return nil
}

func (m *MockContext) showContexts() error {
	return nil
}

func (m *MockContext) showCurrentContext() error {
	return nil
}

func (m *MockContext) showUseContext() error {
	return nil
}

func (m *MockContext) showRemoveContext() error {
	return nil
}

var _ = ginkgo_context.Describe("Test Cloud Context", func() {
	var (
		streams genericclioptions.IOStreams
		o       *ContextOptions
	)
	ginkgo_context.BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		o = &ContextOptions{
			ContextName: "test_context",
			Context: &MockContext{
				IOStreams: streams,
			},
			IOStreams: streams,
		}
	})

	ginkgo_context.AfterEach(func() {
	})

	ginkgo_context.Context("test context", func() {
		Expect(organization.SetCurrentOrgAndContext(&organization.CurrentOrgAndContext{
			CurrentOrganization: "test_org",
			CurrentContext:      "test_context",
		})).Should(BeNil())

		args := []string{"test_context"}
		ginkgo_context.It("test context list ", func() {
			cmd := NewContextListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runList()).Should(Succeed())
		})

		ginkgo_context.It("test context current ", func() {
			cmd := NewContextCurrentCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runCurrent()).Should(Succeed())
		})

		ginkgo_context.It("test context describe ", func() {
			cmd := NewContextDescribeCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runDescribe()).Should(Succeed())
		})

		ginkgo_context.It("test context use ", func() {
			cmd := NewContextUseCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runUse()).Should(Succeed())
		})

		ginkgo_context.It("test context remove ", func() {
			cmd := NewContextRemoveCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runRemove()).Should(Succeed())
		})
	})
})
