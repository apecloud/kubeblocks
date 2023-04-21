/*
Copyright (C) 2022 ApeCloud Co., Ltd

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

package alert

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	clientfake "k8s.io/client-go/rest/fake"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

var _ = Describe("alter", func() {
	var f *cmdtesting.TestFactory
	var s genericclioptions.IOStreams

	BeforeEach(func() {
		f = cmdtesting.NewTestFactory()
		f.Client = &clientfake.RESTClient{}
		s, _, _, _ = genericclioptions.NewTestIOStreams()
	})

	AfterEach(func() {
		f.Cleanup()
	})

	It("create new delete receiver cmd", func() {
		cmd := newDeleteReceiverCmd(f, s)
		Expect(cmd).NotTo(BeNil())
	})

	It("validate", func() {
		o := &deleteReceiverOptions{baseOptions: baseOptions{IOStreams: s}}
		Expect(o.validate([]string{})).Should(HaveOccurred())
		Expect(o.validate([]string{"test"})).Should(Succeed())
	})

	It("run", func() {
		o := &deleteReceiverOptions{baseOptions: mockBaseOptions(s)}
		o.client = testing.FakeClientSet(o.baseOptions.alterConfigMap, o.baseOptions.webhookConfigMap)
		o.names = []string{"receiver-7pb52"}
		Expect(o.run()).Should(Succeed())
	})
})
