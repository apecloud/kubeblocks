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

package alert

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/pkg/cli/testing"
)

var _ = Describe("list smtpserver", func() {
	var f *cmdtesting.TestFactory
	var s genericiooptions.IOStreams

	BeforeEach(func() {
		f = cmdtesting.NewTestFactory()
		f.Client = &clientfake.RESTClient{}
		s, _, _, _ = genericiooptions.NewTestIOStreams()
	})

	AfterEach(func() {
		f.Cleanup()
	})

	It("create new list smtpserver cmd", func() {
		cmd := newListSMTPServerCmd(f, s)
		Expect(cmd).NotTo(BeNil())
	})

	It("run", func() {
		o := &listSMTPServerOptions{baseOptions: mockBaseOptions(s)}
		o.client = testing.FakeClientSet(o.baseOptions.alertConfigMap, o.baseOptions.webhookConfigMap)
		Expect(o.run()).Should(Succeed())
	})
})
