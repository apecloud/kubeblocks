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

package cluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	clientfake "k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"

	"github.com/apecloud/kubeblocks/internal/cli/cluster"
	"github.com/apecloud/kubeblocks/internal/cli/create"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/testing"
)

// write test case to test engine.go
var _ = Describe("create cluster by engine type", func() {
	const (
		engine = cluster.MySQL
	)

	var (
		tf            *cmdtesting.TestFactory
		streams       genericclioptions.IOStreams
		createOptions *create.CreateOptions
	)

	BeforeEach(func() {
		streams, _, _, _ = genericclioptions.NewTestIOStreams()
		tf = testing.NewTestFactory("default")
		tf.Client = &clientfake.RESTClient{}
		createOptions = &create.CreateOptions{
			IOStreams: streams,
			Factory:   tf,
		}
	})

	AfterEach(func() {
		tf.Cleanup()
	})

	It("engine cmd", func() {
		By("create engine commands")
		cmds := buildCreateEngineCmds(createOptions)
		Expect(cmds).ShouldNot(BeNil())
		Expect(cmds[0].HasFlags()).Should(BeTrue())

		By("create engine options")
		o, err := newEngineOptions(createOptions, engine)
		Expect(err).Should(Succeed())
		Expect(o).ShouldNot(BeNil())
		Expect(o.chart).ShouldNot(BeNil())
		Expect(o.schema).ShouldNot(BeNil())

		By("complete")
		cmd := cmds[0]
		o.Format = printer.YAML
		Expect(o.CreateOptions.Complete()).Should(Succeed())
		Expect(o.complete(cmd, nil)).Should(Succeed())
		Expect(o.Name).ShouldNot(BeEmpty())
		Expect(o.values).ShouldNot(BeNil())

		By("validate")
		Expect(o.validate()).Should(Succeed())

		By("run")
		o.DryRun = "client"
		Expect(o.run()).Should(Succeed())
	})
})
