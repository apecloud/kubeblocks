/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package context

import (
	ginkgo_context "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/apecloud/kubeblocks/internal/cli/cmd/organization"
)

type MockContext struct {
	genericiooptions.IOStreams
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
		streams genericiooptions.IOStreams
		o       *ContextOptions
	)
	ginkgo_context.BeforeEach(func() {
		streams, _, _, _ = genericiooptions.NewTestIOStreams()
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
			cmd := newContextListCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runList()).Should(Succeed())
		})

		ginkgo_context.It("test context current ", func() {
			cmd := newContextCurrentCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runCurrent()).Should(Succeed())
		})

		ginkgo_context.It("test context describe ", func() {
			cmd := newContextDescribeCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runDescribe()).Should(Succeed())
		})

		ginkgo_context.It("test context use ", func() {
			cmd := newContextUseCmd(streams)
			Expect(o.complete(args)).Should(Succeed())
			Expect(o.validate(cmd)).Should(Succeed())
			Expect(o.runUse()).Should(Succeed())
		})
	})
})
